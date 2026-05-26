package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Task struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Done        bool       `json:"done"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TaskUpdate carries optional fields for a partial update. A nil pointer
// means "leave unchanged". To clear due_at, set ClearDueAt = true.
type TaskUpdate struct {
	Title       *string
	Description *string
	Done        *bool
	DueAt       *time.Time
	ClearDueAt  bool
}

const taskCols = `id::text, user_id::text, title, description, done, due_at, created_at, updated_at`

func (s *Store) CreateTask(ctx context.Context, userID, title, description string, dueAt *time.Time) (Task, error) {
	var t Task
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO tasks (user_id, title, description, due_at)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING `+taskCols,
		userID, title, description, dueAt,
	).Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Done, &t.DueAt, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (s *Store) ListTasks(ctx context.Context, userID string) ([]Task, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+taskCols+`
		FROM tasks
		WHERE user_id = $1::uuid
		ORDER BY done ASC, COALESCE(due_at, 'infinity'::timestamptz) ASC, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Task, 0)
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Done, &t.DueAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) TaskByID(ctx context.Context, userID, id string) (Task, error) {
	var t Task
	err := s.Pool.QueryRow(ctx, `
		SELECT `+taskCols+`
		FROM tasks WHERE id = $1::uuid AND user_id = $2::uuid
	`, id, userID).Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Done, &t.DueAt, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

func (s *Store) UpdateTask(ctx context.Context, userID, id string, u TaskUpdate) (Task, error) {
	var t Task
	err := s.Pool.QueryRow(ctx, `
		UPDATE tasks SET
			title       = COALESCE($3, title),
			description = COALESCE($4, description),
			done        = COALESCE($5, done),
			due_at      = CASE WHEN $7 THEN NULL ELSE COALESCE($6, due_at) END,
			updated_at  = now()
		WHERE id = $1::uuid AND user_id = $2::uuid
		RETURNING `+taskCols,
		id, userID, u.Title, u.Description, u.Done, u.DueAt, u.ClearDueAt,
	).Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Done, &t.DueAt, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

func (s *Store) DeleteTask(ctx context.Context, userID, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1::uuid AND user_id = $2::uuid`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
