package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type totpRepository struct {
	db *sqlx.DB
}

func newTOTPRepository(db *sqlx.DB) *totpRepository {
	return &totpRepository{db: db}
}

func (r *totpRepository) CreateChallenge(ctx context.Context, challenge model.TOTPChallenge) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "TOTPRepository").Str("action", "CreateChallenge").Msg("create TOTP challenge")

	_, err := r.db.NamedExecContext(ctx,
		`INSERT INTO totp_challenges (id, user_id, created_at, expires_at, attempts, user_agent)
		VALUES (:id, :user_id, :created_at, :expires_at, :attempts, :user_agent)`,
		challenge)
	if err != nil {
		return fmt.Errorf("failed to create TOTP challenge: %w", err)
	}
	return nil
}

func (r *totpRepository) GetChallenge(ctx context.Context, id uuid.UUID) (*model.TOTPChallenge, error) {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "TOTPRepository").Str("action", "GetChallenge").Msg("get TOTP challenge")

	var challenge model.TOTPChallenge
	err := r.db.GetContext(ctx, &challenge, "SELECT * FROM totp_challenges WHERE id = ?", id.String())
	if err != nil {
		return nil, fmt.Errorf("unable to get TOTP challenge by id %s: %w", id, err)
	}
	return &challenge, nil
}

func (r *totpRepository) IncrementChallengeAttempts(ctx context.Context, id uuid.UUID) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "TOTPRepository").Str("action", "IncrementChallengeAttempts").Msg("increment challenge attempts")

	_, err := r.db.ExecContext(ctx, `UPDATE totp_challenges SET attempts = attempts + 1 WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("failed to increment challenge attempts: %w", err)
	}
	return nil
}

func (r *totpRepository) DeleteChallenge(ctx context.Context, id uuid.UUID) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "TOTPRepository").Str("action", "DeleteChallenge").Msg("delete TOTP challenge")

	_, err := r.db.ExecContext(ctx, `DELETE FROM totp_challenges WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("unable to delete TOTP challenge: %w", err)
	}
	return nil
}

func (r *totpRepository) DeleteExpiredChallenges(ctx context.Context) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "TOTPRepository").Str("action", "DeleteExpiredChallenges").Msg("cleanup expired challenges")

	_, err := r.db.ExecContext(ctx, `DELETE FROM totp_challenges WHERE expires_at < ?`, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to delete expired challenges: %w", err)
	}
	return nil
}

func (r *totpRepository) CreateRecoveryCodes(ctx context.Context, codes []model.RecoveryCode) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "TOTPRepository").Str("action", "CreateRecoveryCodes").Int("count", len(codes)).Msg("create recovery codes")

	for _, code := range codes {
		_, err := r.db.NamedExecContext(ctx,
			`INSERT INTO recovery_codes (id, user_id, code_hash, created_at)
			VALUES (:id, :user_id, :code_hash, :created_at)`,
			code)
		if err != nil {
			return fmt.Errorf("failed to create recovery code: %w", err)
		}
	}
	return nil
}

func (r *totpRepository) GetUnusedRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]model.RecoveryCode, error) {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "TOTPRepository").Str("action", "GetUnusedRecoveryCodes").Msg("get unused recovery codes")

	var codes []model.RecoveryCode
	err := r.db.SelectContext(ctx, &codes,
		`SELECT * FROM recovery_codes WHERE user_id = ? AND used_at IS NULL`, userID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get unused recovery codes: %w", err)
	}
	return codes, nil
}

func (r *totpRepository) MarkRecoveryCodeUsed(ctx context.Context, id uuid.UUID) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "TOTPRepository").Str("action", "MarkRecoveryCodeUsed").Msg("mark recovery code as used")

	_, err := r.db.ExecContext(ctx,
		`UPDATE recovery_codes SET used_at = ? WHERE id = ?`, time.Now().UTC(), id.String())
	if err != nil {
		return fmt.Errorf("failed to mark recovery code as used: %w", err)
	}
	return nil
}

func (r *totpRepository) DeleteRecoveryCodesByUserID(ctx context.Context, userID uuid.UUID) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "TOTPRepository").Str("action", "DeleteRecoveryCodesByUserID").Msg("delete all recovery codes for user")

	_, err := r.db.ExecContext(ctx,
		`DELETE FROM recovery_codes WHERE user_id = ?`, userID.String())
	if err != nil {
		return fmt.Errorf("failed to delete recovery codes: %w", err)
	}
	return nil
}
