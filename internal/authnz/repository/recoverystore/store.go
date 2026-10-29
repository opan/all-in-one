// Package recoverystore keeps temporary passwords and pending email
// verifications (RFC-003). The queries are plain SQL with ? placeholders,
// rebound per driver, so one implementation serves SQLite and Postgres.
package recoverystore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/all-in-one/internal/authnz/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PasswordResets struct {
	db *sqlx.DB
}

func NewPasswordResets(db *sqlx.DB) *PasswordResets {
	return &PasswordResets{db: db}
}

// Create stores a new temporary password and voids the user's earlier ones,
// so only the most recent email works.
func (r *PasswordResets) Create(ctx context.Context, reset model.PasswordReset) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE password_resets SET voided_at = ?
		WHERE user_id = ? AND used_at IS NULL AND voided_at IS NULL`), time.Now().UTC(), reset.UserID.String()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO password_resets
		(id, user_id, temp_password_hash, expires_at, attempts, issued_by, created_at) VALUES (?, ?, ?, ?, 0, ?, ?)`),
		reset.ID.String(), reset.UserID.String(), reset.TempPasswordHash, reset.ExpiresAt.UTC(), reset.IssuedBy, reset.CreatedAt.UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

// Active returns the user's temporary passwords that can still be used.
func (r *PasswordResets) Active(ctx context.Context, userID uuid.UUID) ([]model.PasswordReset, error) {
	var resets []model.PasswordReset
	err := r.db.SelectContext(ctx, &resets, r.db.Rebind(`SELECT id, user_id, temp_password_hash, expires_at, attempts,
		used_at, voided_at, issued_by, created_at FROM password_resets
		WHERE user_id = ? AND used_at IS NULL AND voided_at IS NULL AND expires_at > ?
		ORDER BY created_at DESC`), userID.String(), time.Now().UTC())
	return resets, err
}

// MarkUsed burns a temporary password. It reports false when the password was
// used or voided in the meantime, so two logins can't both use it.
func (r *PasswordResets) MarkUsed(ctx context.Context, id uuid.UUID) (bool, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE password_resets SET used_at = ?
		WHERE id = ? AND used_at IS NULL AND voided_at IS NULL`), time.Now().UTC(), id.String())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// RecordFailedAttempt counts a wrong temporary password and voids it once
// maxAttempts is reached.
func (r *PasswordResets) RecordFailedAttempt(ctx context.Context, id uuid.UUID, maxAttempts int) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE password_resets SET attempts = attempts + 1,
		voided_at = CASE WHEN attempts + 1 >= ? THEN ? ELSE voided_at END
		WHERE id = ? AND used_at IS NULL AND voided_at IS NULL`), maxAttempts, time.Now().UTC(), id.String())
	return err
}

func (r *PasswordResets) VoidActive(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE password_resets SET voided_at = ?
		WHERE user_id = ? AND used_at IS NULL AND voided_at IS NULL`), time.Now().UTC(), userID.String())
	return err
}

// CountSince counts temporary passwords issued to the user since t, used for
// the per-account email limits.
func (r *PasswordResets) CountSince(ctx context.Context, userID uuid.UUID, t time.Time) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, r.db.Rebind(`SELECT COUNT(*) FROM password_resets WHERE user_id = ? AND created_at > ?`),
		userID.String(), t.UTC())
	return n, err
}

type EmailVerifications struct {
	db *sqlx.DB
}

func NewEmailVerifications(db *sqlx.DB) *EmailVerifications {
	return &EmailVerifications{db: db}
}

// Create stores a pending email and expires the user's earlier pending ones,
// so only the latest link works. Rows are kept for the per-account limits.
func (r *EmailVerifications) Create(ctx context.Context, v model.EmailVerification) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, tx.Rebind(`UPDATE email_verifications SET expires_at = ?
		WHERE user_id = ? AND used_at IS NULL AND expires_at > ?`), now, v.UserID.String(), now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO email_verifications
		(id, user_id, email, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`),
		v.ID.String(), v.UserID.String(), v.Email, v.TokenHash, v.ExpiresAt.UTC(), v.CreatedAt.UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

const verificationColumns = `id, user_id, email, token_hash, expires_at, used_at, created_at`

// FindUsable returns the unused, unexpired verification with this token hash,
// or sql.ErrNoRows.
func (r *EmailVerifications) FindUsable(ctx context.Context, tokenHash string) (model.EmailVerification, error) {
	var v model.EmailVerification
	err := r.db.GetContext(ctx, &v, r.db.Rebind(`SELECT `+verificationColumns+` FROM email_verifications
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`), tokenHash, time.Now().UTC())
	return v, err
}

// Pending returns the user's latest usable verification, or nil.
func (r *EmailVerifications) Pending(ctx context.Context, userID uuid.UUID) (*model.EmailVerification, error) {
	var v model.EmailVerification
	err := r.db.GetContext(ctx, &v, r.db.Rebind(`SELECT `+verificationColumns+` FROM email_verifications
		WHERE user_id = ? AND used_at IS NULL AND expires_at > ? ORDER BY created_at DESC LIMIT 1`), userID.String(), time.Now().UTC())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *EmailVerifications) MarkUsed(ctx context.Context, id uuid.UUID) (bool, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE email_verifications SET used_at = ? WHERE id = ? AND used_at IS NULL`),
		time.Now().UTC(), id.String())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (r *EmailVerifications) CountSince(ctx context.Context, userID uuid.UUID, t time.Time) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, r.db.Rebind(`SELECT COUNT(*) FROM email_verifications WHERE user_id = ? AND created_at > ?`),
		userID.String(), t.UTC())
	return n, err
}
