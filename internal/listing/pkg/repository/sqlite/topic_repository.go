package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type topicRepository struct {
	db *sqlx.DB
}

func newTopicRepository(db *sqlx.DB) *topicRepository {
	return &topicRepository{db: db}
}

func (r *topicRepository) GetAll() ([]model.Topic, error) {
	rows, err := r.db.Query(`
		SELECT id, name, description, created_at, updated_at 
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

		err := rows.Scan(&topic.ID, &topic.Name, &topic.Description, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("unable to scan topic row: %w", err)
		}

		// Parse timestamps
		topic.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		topic.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		topics = append(topics, topic)
	}

	return topics, nil
}

func (r *topicRepository) Get(id int) (model.Topic, error) {
	var topic model.Topic
	var createdAt, updatedAt string

	err := r.db.QueryRow(`
		SELECT id, name, description, created_at, updated_at 
		FROM topics
		WHERE id = ?
	`, id).Scan(&topic.ID, &topic.Name, &topic.Description, &createdAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return topic, fmt.Errorf("topic with id %d not found", id)
		}
		return topic, fmt.Errorf("unable to fetch topic from db: %w", err)
	}

	// Parse timestamps
	topic.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	topic.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return topic, nil
}

func (r *topicRepository) Create(topic model.Topic) (model.Topic, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := r.db.Exec(`
		INSERT INTO topics (name, description, created_at, updated_at) 
		VALUES (?, ?, ?, ?)
	`, topic.Name, topic.Description, now, now)

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

func (r *topicRepository) Update(id int, topic model.Topic) (model.Topic, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.Exec(`
		UPDATE topics 
		SET name = ?, description = ?, updated_at = ? 
		WHERE id = ?
	`, topic.Name, topic.Description, now, id)

	if err != nil {
		return model.Topic{}, fmt.Errorf("unable to update topic in db: %w", err)
	}

	topic.ID = id
	topic.UpdatedAt, _ = time.Parse(time.RFC3339, now)

	return topic, nil
}

func (r *topicRepository) Delete(id int) error {
	_, err := r.db.Exec(`
		DELETE FROM topics 
		WHERE id = ?
	`, id)

	if err != nil {
		return fmt.Errorf("unable to delete topic from db: %w", err)
	}

	return nil
}
