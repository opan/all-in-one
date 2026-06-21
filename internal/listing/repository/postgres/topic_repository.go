package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/all-in-one/internal/listing/model"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/query"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type queryOptions struct {
	trx *sqlx.Tx
	db  *sqlx.DB
}

func (q *queryOptions) Commit() error {
	if q.trx != nil {
		return q.trx.Commit()
	}
	return nil
}

func (q *queryOptions) Rollback() error {
	if q.trx != nil {
		return q.trx.Rollback()
	}
	return nil
}

type topicRepository struct {
	db *sqlx.DB
}

func newTopicRepository(db *sqlx.DB) *topicRepository {
	return &topicRepository{db: db}
}

type Execer interface {
	sqlx.ExtContext
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
}

func getExecCtx(db *sqlx.DB, opts ...query.QueryOptions) Execer {
	for _, opt := range opts {
		if qo, ok := opt.(*queryOptions); ok && qo.trx != nil {
			return qo.trx
		}
	}
	return db
}

func (r *topicRepository) CreateTrx(ctx context.Context) (query.QueryOptions, error) {
	trx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}
	return &queryOptions{trx: trx, db: r.db}, nil
}

func (r *topicRepository) GetAll(ctx context.Context, userID uuid.UUID) ([]model.Topic, error) {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "TopicRepo").Str("action", "GetAll").Str("user_id", userID.String()).Msg("fetching all topics from database")

	var topics []model.Topic
	if err := r.db.SelectContext(ctx, &topics, "SELECT * FROM topics WHERE user_id = $1", userID.String()); err != nil {
		return nil, err
	}
	return topics, nil
}

func (r *topicRepository) Get(ctx context.Context, id int) (model.Topic, error) {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "TopicRepo").Str("action", "Get").Int("topic_id", id).Msg("fetching topic from database")

	var topic model.Topic
	if err := r.db.GetContext(ctx, &topic, "SELECT * FROM topics WHERE id = $1", id); err != nil {
		return model.Topic{}, fmt.Errorf("unable to get topic from db: %w", err)
	}
	return topic, nil
}

func (r *topicRepository) Create(ctx context.Context, topic model.Topic) (model.Topic, error) {
	now := time.Now().UTC()
	var id int
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO topics (user_id, name, description, created_at, updated_at, form_schema)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, topic.UserID.String(), topic.Name, topic.Description, now, now, topic.FormSchema).Scan(&id)
	if err != nil {
		return model.Topic{}, fmt.Errorf("unable to insert topic into db: %w", err)
	}
	topic.ID = id
	topic.CreatedAt = now
	topic.UpdatedAt = now
	return topic, nil
}

func (r *topicRepository) Update(ctx context.Context, id int, topic model.Topic) (model.Topic, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE topics
		SET name = $1, description = $2, updated_at = $3, form_schema = $4
		WHERE id = $5
	`, topic.Name, topic.Description, now, topic.FormSchema, id)
	if err != nil {
		return model.Topic{}, fmt.Errorf("unable to update topic in db: %w", err)
	}
	topic.ID = id
	topic.UpdatedAt = now
	return topic, nil
}

func (r *topicRepository) Delete(ctx context.Context, id int, opts ...query.QueryOptions) error {
	exec := getExecCtx(r.db, opts...)
	_, err := exec.ExecContext(ctx, `DELETE FROM topics WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("unable to delete topic from db: %w", err)
	}
	return nil
}
