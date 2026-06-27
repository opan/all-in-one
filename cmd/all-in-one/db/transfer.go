package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// TransferOpts configures a cross-backend data migration.
type TransferOpts struct {
	// Direction is either "sqlite-to-pg" or "pg-to-sqlite".
	Direction string
	Config    config.Config
	Logger    zerolog.Logger
}

// RunTransfer copies all application data from one backend to the other.
// Both databases must have all schema migrations applied before running.
// The destination must be empty; duplicate rows cause constraint failures.
func RunTransfer(ctx context.Context, opts TransferOpts) error {
	log := opts.Logger

	var srcDB, dstDB *sqlx.DB
	var err error

	switch opts.Direction {
	case "sqlite-to-pg":
		srcDB, err = sqlx.Open("sqlite3", opts.Config.Storage.SQLite.DBPath+"?_foreign_keys=on")
		if err != nil {
			return fmt.Errorf("open sqlite source: %w", err)
		}
		dstDB, err = sqlx.Open("postgres", opts.Config.Storage.Postgres.DSN())
		if err != nil {
			return fmt.Errorf("open postgres destination: %w", err)
		}
	case "pg-to-sqlite":
		srcDB, err = sqlx.Open("postgres", opts.Config.Storage.Postgres.DSN())
		if err != nil {
			return fmt.Errorf("open postgres source: %w", err)
		}
		dstDB, err = sqlx.Open("sqlite3", opts.Config.Storage.SQLite.DBPath+"?_foreign_keys=on")
		if err != nil {
			return fmt.Errorf("open sqlite destination: %w", err)
		}
	default:
		return fmt.Errorf("unknown direction %q: use sqlite-to-pg or pg-to-sqlite", opts.Direction)
	}
	defer srcDB.Close()
	defer dstDB.Close()

	if err := srcDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping source: %w", err)
	}
	if err := dstDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping destination: %w", err)
	}

	log.Info().Str("direction", opts.Direction).Msg("reading all source data into memory")
	data, err := readAll(ctx, srcDB)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	logTransferSummary(log, data)

	log.Info().Msg("writing to destination within a single transaction")
	total, err := writeAll(ctx, dstDB, data, opts.Direction, log)
	if err != nil {
		return fmt.Errorf("write destination: %w", err)
	}

	log.Info().Int("total_rows", total).Msg("transfer complete")
	return nil
}

// --- custom scanner types ---

// nullableTime scans SQL timestamps from SQLite (string) or PostgreSQL (time.Time).
type nullableTime struct {
	T *time.Time
}

func (n *nullableTime) Scan(v any) error {
	if v == nil {
		n.T = nil
		return nil
	}
	switch val := v.(type) {
	case time.Time:
		t := val.UTC()
		n.T = &t
	case string:
		t, err := flexParseTime(val)
		if err != nil {
			return err
		}
		n.T = &t
	case []byte:
		t, err := flexParseTime(string(val))
		if err != nil {
			return err
		}
		n.T = &t
	default:
		return fmt.Errorf("nullableTime: unsupported type %T", v)
	}
	return nil
}

// boolInt scans BOOLEAN (PostgreSQL) or INTEGER 0/1 (SQLite) into a Go bool.
type boolInt struct {
	V bool
}

func (b *boolInt) Scan(v any) error {
	switch val := v.(type) {
	case bool:
		b.V = val
	case int64:
		b.V = val != 0
	case int:
		b.V = val != 0
	default:
		return fmt.Errorf("boolInt: unsupported type %T", v)
	}
	return nil
}

func flexParseTime(s string) (time.Time, error) {
	for _, f := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %q", s)
}

// boolForDst returns 1/0 for SQLite destinations and bool for PostgreSQL.
func boolForDst(v bool, direction string) any {
	if direction == "pg-to-sqlite" {
		if v {
			return 1
		}
		return 0
	}
	return v
}

// insertQuery builds an INSERT statement with the correct placeholder style.
func insertQuery(table string, cols []string, direction string) string {
	ph := make([]string, len(cols))
	if direction == "sqlite-to-pg" {
		for i := range cols {
			ph[i] = fmt.Sprintf("$%d", i+1)
		}
	} else {
		for i := range cols {
			ph[i] = "?"
		}
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(cols, ", "), strings.Join(ph, ", "))
}

// --- per-table row types ---

type userRow struct {
	ID                  string         `db:"id"`
	Username            string         `db:"username"`
	Email               string         `db:"email"`
	Name                sql.NullString `db:"name"`
	PasswordHash        string         `db:"password_hash"`
	LastLogin           nullableTime   `db:"last_login"`
	CreatedAt           nullableTime   `db:"created_at"`
	UpdatedAt           nullableTime   `db:"updated_at"`
	TOTPEnabled         boolInt        `db:"totp_enabled"`
	TOTPSecretEncrypted sql.NullString `db:"totp_secret_encrypted"`
	TOTPVerifiedAt      nullableTime   `db:"totp_verified_at"`
}

type sessionRow struct {
	ID                 string         `db:"id"`
	UserID             string         `db:"user_id"`
	CreatedAt          nullableTime   `db:"created_at"`
	UserAgent          sql.NullString `db:"user_agent"`
	AccessTokenExpiry  sql.NullInt64  `db:"access_token_expiry"`
	RefreshTokenExpiry sql.NullInt64  `db:"refresh_token_expiry"`
}

type topicRow struct {
	ID          int64          `db:"id"`
	UserID      string         `db:"user_id"`
	Name        string         `db:"name"`
	Description sql.NullString `db:"description"`
	FormSchema  sql.NullString `db:"form_schema"`
	CreatedAt   nullableTime   `db:"created_at"`
	UpdatedAt   nullableTime   `db:"updated_at"`
}

type itemRow struct {
	ID               int64          `db:"id"`
	TopicID          sql.NullInt64  `db:"topic_id"`
	Title            string         `db:"title"`
	Description      sql.NullString `db:"description"`
	FormSchemaValues sql.NullString `db:"form_schema_values"`
	CreatedAt        nullableTime   `db:"created_at"`
	UpdatedAt        nullableTime   `db:"updated_at"`
}

type chatSessionRow struct {
	ID              string       `db:"id"`
	ParticipantHash string       `db:"participant_hash"`
	Status          string       `db:"status"`
	CreatedAt       nullableTime `db:"created_at"`
	UpdatedAt       nullableTime `db:"updated_at"`
	CreatedBy       string       `db:"created_by"`
}

type chatParticipantRow struct {
	SessionID string       `db:"session_id"`
	UserID    string       `db:"user_id"`
	JoinedAt  nullableTime `db:"joined_at"`
	LeftAt    nullableTime `db:"left_at"`
}

type chatMessageRow struct {
	ID            string       `db:"id"`
	ChatSessionID string       `db:"chat_session_id"`
	UserID        string       `db:"user_id"`
	Message       string       `db:"message"`
	CreatedAt     nullableTime `db:"created_at"`
	SentAt        nullableTime `db:"sent_at"`
}

type chatInviteRow struct {
	ID        string         `db:"id"`
	BatchID   string         `db:"batch_id"`
	InviterID string         `db:"inviter_id"`
	InviteeID string         `db:"invitee_id"`
	SessionID sql.NullString `db:"session_id"`
	Status    string         `db:"status"`
	CreatedAt nullableTime   `db:"created_at"`
	UpdatedAt nullableTime   `db:"updated_at"`
}

type recoveryCodeRow struct {
	ID        string       `db:"id"`
	UserID    string       `db:"user_id"`
	CodeHash  string       `db:"code_hash"`
	UsedAt    nullableTime `db:"used_at"`
	CreatedAt nullableTime `db:"created_at"`
}

type totpChallengeRow struct {
	ID        string         `db:"id"`
	UserID    string         `db:"user_id"`
	CreatedAt nullableTime   `db:"created_at"`
	ExpiresAt nullableTime   `db:"expires_at"`
	Attempts  int            `db:"attempts"`
	UserAgent sql.NullString `db:"user_agent"`
}

type shortLinkRow struct {
	ID             string       `db:"id"`
	Code           string       `db:"code"`
	TargetURL      string       `db:"target_url"`
	CreatedAt      nullableTime `db:"created_at"`
	ExpiresAt      nullableTime `db:"expires_at"`
	IsActive       boolInt      `db:"is_active"`
	ClickCount     int          `db:"click_count"`
	LastAccessedAt nullableTime `db:"last_accessed_at"`
}

type shortLinkOwnerRow struct {
	Code   string `db:"code"`
	UserID string `db:"user_id"`
}

// transferData holds all rows read from the source database.
type transferData struct {
	users            []userRow
	sessions         []sessionRow
	topics           []topicRow
	items            []itemRow
	chatSessions     []chatSessionRow
	chatParticipants []chatParticipantRow
	chatMessages     []chatMessageRow
	chatInvites      []chatInviteRow
	recoveryCodes    []recoveryCodeRow
	totpChallenges   []totpChallengeRow
	shortLinks       []shortLinkRow
	shortLinkOwners  []shortLinkOwnerRow
}

func readAll(ctx context.Context, src *sqlx.DB) (*transferData, error) {
	d := &transferData{}

	if err := queryInto(ctx, src, &d.users, `
		SELECT id, username, email, name, password_hash, last_login, created_at, updated_at,
		       totp_enabled, totp_secret_encrypted, totp_verified_at
		FROM users`); err != nil {
		return nil, fmt.Errorf("users: %w", err)
	}

	if err := queryInto(ctx, src, &d.sessions, `
		SELECT id, user_id, created_at, user_agent, access_token_expiry, refresh_token_expiry
		FROM sessions`); err != nil {
		return nil, fmt.Errorf("sessions: %w", err)
	}

	if err := queryInto(ctx, src, &d.topics, `
		SELECT id, user_id, name, description, form_schema, created_at, updated_at
		FROM topics`); err != nil {
		return nil, fmt.Errorf("topics: %w", err)
	}

	if err := queryInto(ctx, src, &d.items, `
		SELECT id, topic_id, title, description, form_schema_values, created_at, updated_at
		FROM items`); err != nil {
		return nil, fmt.Errorf("items: %w", err)
	}

	if err := queryInto(ctx, src, &d.chatSessions, `
		SELECT id, participant_hash, status, created_at, updated_at, created_by
		FROM chat_sessions`); err != nil {
		return nil, fmt.Errorf("chat_sessions: %w", err)
	}

	if err := queryInto(ctx, src, &d.chatParticipants, `
		SELECT session_id, user_id, joined_at, left_at
		FROM chat_session_participants`); err != nil {
		return nil, fmt.Errorf("chat_session_participants: %w", err)
	}

	if err := queryInto(ctx, src, &d.chatMessages, `
		SELECT id, chat_session_id, user_id, message, created_at, sent_at
		FROM chat_messages`); err != nil {
		return nil, fmt.Errorf("chat_messages: %w", err)
	}

	if err := queryInto(ctx, src, &d.chatInvites, `
		SELECT id, batch_id, inviter_id, invitee_id, session_id, status, created_at, updated_at
		FROM chat_invites`); err != nil {
		return nil, fmt.Errorf("chat_invites: %w", err)
	}

	if err := queryInto(ctx, src, &d.recoveryCodes, `
		SELECT id, user_id, code_hash, used_at, created_at
		FROM recovery_codes`); err != nil {
		return nil, fmt.Errorf("recovery_codes: %w", err)
	}

	if err := queryInto(ctx, src, &d.totpChallenges, `
		SELECT id, user_id, created_at, expires_at, attempts, user_agent
		FROM totp_challenges`); err != nil {
		return nil, fmt.Errorf("totp_challenges: %w", err)
	}

	if err := queryInto(ctx, src, &d.shortLinks, `
		SELECT id, code, target_url, created_at, expires_at, is_active, click_count, last_accessed_at
		FROM short_links`); err != nil {
		return nil, fmt.Errorf("short_links: %w", err)
	}

	if err := queryInto(ctx, src, &d.shortLinkOwners, `
		SELECT code, user_id
		FROM short_link_owners`); err != nil {
		return nil, fmt.Errorf("short_link_owners: %w", err)
	}

	return d, nil
}

// queryInto reads all rows from query into a slice of structs via StructScan.
// dest must be a pointer to a slice of structs with db tags.
func queryInto[T any](ctx context.Context, db *sqlx.DB, dest *[]T, query string) error {
	rows, err := db.QueryxContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r T
		if err := rows.StructScan(&r); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		*dest = append(*dest, r)
	}
	return rows.Err()
}

func writeAll(ctx context.Context, dst *sqlx.DB, data *transferData, direction string, log zerolog.Logger) (int, error) {
	tx, err := dst.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	total := 0

	// users
	q := insertQuery("users", []string{
		"id", "username", "email", "name", "password_hash",
		"last_login", "created_at", "updated_at",
		"totp_enabled", "totp_secret_encrypted", "totp_verified_at",
	}, direction)
	for _, r := range data.users {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.Username, r.Email, r.Name, r.PasswordHash,
			r.LastLogin.T, r.CreatedAt.T, r.UpdatedAt.T,
			boolForDst(r.TOTPEnabled.V, direction),
			r.TOTPSecretEncrypted, r.TOTPVerifiedAt.T,
		); err != nil {
			return total, fmt.Errorf("insert user %s: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "users").Int("rows", len(data.users)).Msg("transferred")
	total += len(data.users)

	// sessions
	q = insertQuery("sessions", []string{
		"id", "user_id", "created_at", "user_agent",
		"access_token_expiry", "refresh_token_expiry",
	}, direction)
	for _, r := range data.sessions {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.UserID, r.CreatedAt.T, r.UserAgent,
			r.AccessTokenExpiry, r.RefreshTokenExpiry,
		); err != nil {
			return total, fmt.Errorf("insert session %s: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "sessions").Int("rows", len(data.sessions)).Msg("transferred")
	total += len(data.sessions)

	// topics
	q = insertQuery("topics", []string{
		"id", "user_id", "name", "description", "form_schema", "created_at", "updated_at",
	}, direction)
	for _, r := range data.topics {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.UserID, r.Name, r.Description, r.FormSchema, r.CreatedAt.T, r.UpdatedAt.T,
		); err != nil {
			return total, fmt.Errorf("insert topic %d: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "topics").Int("rows", len(data.topics)).Msg("transferred")
	total += len(data.topics)

	// items
	q = insertQuery("items", []string{
		"id", "topic_id", "title", "description", "form_schema_values", "created_at", "updated_at",
	}, direction)
	for _, r := range data.items {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.TopicID, r.Title, r.Description, r.FormSchemaValues, r.CreatedAt.T, r.UpdatedAt.T,
		); err != nil {
			return total, fmt.Errorf("insert item %d: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "items").Int("rows", len(data.items)).Msg("transferred")
	total += len(data.items)

	// chat_sessions
	q = insertQuery("chat_sessions", []string{
		"id", "participant_hash", "status", "created_at", "updated_at", "created_by",
	}, direction)
	for _, r := range data.chatSessions {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.ParticipantHash, r.Status, r.CreatedAt.T, r.UpdatedAt.T, r.CreatedBy,
		); err != nil {
			return total, fmt.Errorf("insert chat_session %s: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "chat_sessions").Int("rows", len(data.chatSessions)).Msg("transferred")
	total += len(data.chatSessions)

	// chat_session_participants
	q = insertQuery("chat_session_participants", []string{
		"session_id", "user_id", "joined_at", "left_at",
	}, direction)
	for _, r := range data.chatParticipants {
		if _, err := tx.ExecContext(ctx, q,
			r.SessionID, r.UserID, r.JoinedAt.T, r.LeftAt.T,
		); err != nil {
			return total, fmt.Errorf("insert chat_participant (%s,%s): %w", r.SessionID, r.UserID, err)
		}
	}
	log.Info().Str("table", "chat_session_participants").Int("rows", len(data.chatParticipants)).Msg("transferred")
	total += len(data.chatParticipants)

	// chat_messages
	q = insertQuery("chat_messages", []string{
		"id", "chat_session_id", "user_id", "message", "created_at", "sent_at",
	}, direction)
	for _, r := range data.chatMessages {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.ChatSessionID, r.UserID, r.Message, r.CreatedAt.T, r.SentAt.T,
		); err != nil {
			return total, fmt.Errorf("insert chat_message %s: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "chat_messages").Int("rows", len(data.chatMessages)).Msg("transferred")
	total += len(data.chatMessages)

	// chat_invites
	q = insertQuery("chat_invites", []string{
		"id", "batch_id", "inviter_id", "invitee_id", "session_id", "status", "created_at", "updated_at",
	}, direction)
	for _, r := range data.chatInvites {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.BatchID, r.InviterID, r.InviteeID, r.SessionID, r.Status, r.CreatedAt.T, r.UpdatedAt.T,
		); err != nil {
			return total, fmt.Errorf("insert chat_invite %s: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "chat_invites").Int("rows", len(data.chatInvites)).Msg("transferred")
	total += len(data.chatInvites)

	// recovery_codes
	q = insertQuery("recovery_codes", []string{
		"id", "user_id", "code_hash", "used_at", "created_at",
	}, direction)
	for _, r := range data.recoveryCodes {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.UserID, r.CodeHash, r.UsedAt.T, r.CreatedAt.T,
		); err != nil {
			return total, fmt.Errorf("insert recovery_code %s: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "recovery_codes").Int("rows", len(data.recoveryCodes)).Msg("transferred")
	total += len(data.recoveryCodes)

	// totp_challenges
	q = insertQuery("totp_challenges", []string{
		"id", "user_id", "created_at", "expires_at", "attempts", "user_agent",
	}, direction)
	for _, r := range data.totpChallenges {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.UserID, r.CreatedAt.T, r.ExpiresAt.T, r.Attempts, r.UserAgent,
		); err != nil {
			return total, fmt.Errorf("insert totp_challenge %s: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "totp_challenges").Int("rows", len(data.totpChallenges)).Msg("transferred")
	total += len(data.totpChallenges)

	// short_links
	q = insertQuery("short_links", []string{
		"id", "code", "target_url", "created_at", "expires_at", "is_active", "click_count", "last_accessed_at",
	}, direction)
	for _, r := range data.shortLinks {
		if _, err := tx.ExecContext(ctx, q,
			r.ID, r.Code, r.TargetURL, r.CreatedAt.T, r.ExpiresAt.T,
			boolForDst(r.IsActive.V, direction), r.ClickCount, r.LastAccessedAt.T,
		); err != nil {
			return total, fmt.Errorf("insert short_link %s: %w", r.ID, err)
		}
	}
	log.Info().Str("table", "short_links").Int("rows", len(data.shortLinks)).Msg("transferred")
	total += len(data.shortLinks)

	// short_link_owners
	q = insertQuery("short_link_owners", []string{"code", "user_id"}, direction)
	for _, r := range data.shortLinkOwners {
		if _, err := tx.ExecContext(ctx, q, r.Code, r.UserID); err != nil {
			return total, fmt.Errorf("insert short_link_owner %s: %w", r.Code, err)
		}
	}
	log.Info().Str("table", "short_link_owners").Int("rows", len(data.shortLinkOwners)).Msg("transferred")
	total += len(data.shortLinkOwners)

	// After bulk-inserting explicit IDs into PostgreSQL IDENTITY columns, advance
	// the sequences so future INSERTs without explicit IDs don't collide.
	if direction == "sqlite-to-pg" {
		for _, q := range []string{
			`SELECT setval(pg_get_serial_sequence('topics', 'id'), COALESCE((SELECT MAX(id) FROM topics), 0))`,
			`SELECT setval(pg_get_serial_sequence('items', 'id'), COALESCE((SELECT MAX(id) FROM items), 0))`,
		} {
			if _, err := tx.ExecContext(ctx, q); err != nil {
				return total, fmt.Errorf("reset sequence: %w", err)
			}
		}
		log.Info().Msg("reset topics/items identity sequences")
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return total, nil
}

func logTransferSummary(log zerolog.Logger, d *transferData) {
	log.Info().
		Int("users", len(d.users)).
		Int("sessions", len(d.sessions)).
		Int("topics", len(d.topics)).
		Int("items", len(d.items)).
		Int("chat_sessions", len(d.chatSessions)).
		Int("chat_participants", len(d.chatParticipants)).
		Int("chat_messages", len(d.chatMessages)).
		Int("chat_invites", len(d.chatInvites)).
		Int("recovery_codes", len(d.recoveryCodes)).
		Int("totp_challenges", len(d.totpChallenges)).
		Int("short_links", len(d.shortLinks)).
		Int("short_link_owners", len(d.shortLinkOwners)).
		Msg("source data summary")
}
