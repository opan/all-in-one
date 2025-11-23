package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/query"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
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

// getExecutor returns the appropriate executor (transaction or database) from options
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

func (r *topicRepository) GetAll(ctx context.Context) ([]model.Topic, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, form_schema, created_at, updated_at 
		FROM topics
		ORDER BY id
	`)

	if err != nil {
		return nil, fmt.Errorf("unable to fetch topics from db: %w", err)
	}

	defer rows.Close()

	var topics []model.Topic
	for rows.Next() {
		var topic model.Topic
		var createdAt, updatedAt string
		var formSchemaJSON sql.NullString

		err := rows.Scan(&topic.ID, &topic.Name, &topic.Description, &formSchemaJSON, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("unable to scan topic row: %w", err)
		}

		// Parse form schema - always initialize it, even if NULL
		if formSchemaJSON.Valid {
			if err := topic.FormSchema.Scan(formSchemaJSON.String); err != nil {
				return nil, fmt.Errorf("unable to parse form_schema: %w", err)
			}
		} else {
			// Initialize with empty schema when NULL
			topic.FormSchema.Scan(nil)
		}

		// Parse timestamps
		topic.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		topic.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		topics = append(topics, topic)
	}

	return topics, nil
}

func (r *topicRepository) Get(ctx context.Context, id int) (model.Topic, error) {
	var topic model.Topic
	var createdAt, updatedAt string
	var formSchemaJSON sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, form_schema, created_at, updated_at 
		FROM topics
		WHERE id = ?
	`, id).Scan(&topic.ID, &topic.Name, &topic.Description, &formSchemaJSON, &createdAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return topic, fmt.Errorf("topic with id %d not found", id)
		}
		return topic, fmt.Errorf("unable to fetch topic from db: %w", err)
	}

	// Parse form schema - always initialize it, even if NULL
	if formSchemaJSON.Valid {
		if err := topic.FormSchema.Scan(formSchemaJSON.String); err != nil {
			return topic, fmt.Errorf("unable to parse form_schema: %w", err)
		}
	} else {
		// Initialize with empty schema when NULL
		topic.FormSchema.Scan(nil)
	}

	// Parse timestamps
	topic.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	topic.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return topic, nil
}

func (r *topicRepository) Create(ctx context.Context, topic model.Topic) (model.Topic, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Convert FormSchema to JSON
	formSchemaJSON, err := topic.FormSchema.Value()
	if err != nil {
		return model.Topic{}, fmt.Errorf("unable to marshal form_schema: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO topics (name, description, form_schema, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?)
	`, topic.Name, topic.Description, formSchemaJSON, now, now)

	if err != nil {
		return model.Topic{}, fmt.Errorf("unable to insert topic into db: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.Topic{}, fmt.Errorf("unable to get last insert id: %w", err)
	}

	topic.ID = int(id)
	topic.CreatedAt, _ = time.Parse(time.RFC3339, now)
	topic.UpdatedAt, _ = time.Parse(time.RFC3339, now)

	return topic, nil
}

func (r *topicRepository) Update(ctx context.Context, id int, topic model.Topic) (model.Topic, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Convert FormSchema to JSON
	formSchemaJSON, err := topic.FormSchema.Value()
	if err != nil {
		return model.Topic{}, fmt.Errorf("unable to marshal form_schema: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE topics 
		SET name = ?, description = ?, form_schema = ?, updated_at = ? 
		WHERE id = ?
	`, topic.Name, topic.Description, formSchemaJSON, now, id)

	if err != nil {
		return model.Topic{}, fmt.Errorf("unable to update topic in db: %w", err)
	}

	topic.ID = id
	topic.UpdatedAt, _ = time.Parse(time.RFC3339, now)

	return topic, nil
}

func (r *topicRepository) Delete(ctx context.Context, id int, opts ...query.QueryOptions) error {
	exec := getExecCtx(r.db, opts...)

	_, err := exec.ExecContext(ctx, `
		DELETE FROM topics 
		WHERE id = ?
	`, id)

	if err != nil {
		return fmt.Errorf("unable to delete topic from db: %w", err)
	}

	return nil
}
