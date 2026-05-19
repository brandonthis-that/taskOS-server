package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/brandonthis-that/taskOS-server/internal/models"
)

//go:embed migrations.sql
var migrationsFS embed.FS

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrForbidden    = errors.New("forbidden")
	ErrInvalidInput = errors.New("invalid input")
)

// Store persists taskOS domain data.
type Store struct {
	db *sql.DB
}

// Open connects to PostgreSQL and applies migrations.
func Open(databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	sqlBytes, err := migrationsFS.ReadFile("migrations.sql")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(sqlBytes)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateUser(ctx context.Context, id, username, apiKeyLookup, apiKeyHash string, createdAt time.Time) (*models.User, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, username, api_key_lookup, api_key_hash, created_at) VALUES ($1, $2, $3, $4, $5)`,
		id, username, apiKeyLookup, apiKeyHash, createdAt.UTC(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &models.User{ID: id, Username: username, CreatedAt: createdAt}, nil
}

func (s *Store) UserByAPIKeyLookup(ctx context.Context, lookup string) (*models.User, string, error) {
	var u models.User
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, api_key_hash, created_at FROM users WHERE api_key_lookup = $1`,
		lookup,
	).Scan(&u.ID, &u.Username, &hash, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	return &u, hash, nil
}

func (s *Store) UserByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, created_at FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) UserByID(ctx context.Context, id string) (*models.User, error) {
	var u models.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Username, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateTask(ctx context.Context, t *models.Task) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tasks (id, owner_id, assignee_id, created_by_id, title, description, status, due_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		t.ID, t.OwnerID, t.AssigneeID, t.CreatedByID, t.Title, t.Description, t.Status,
		timePtr(t.DueAt), t.CreatedAt.UTC(), t.UpdatedAt.UTC(),
	)
	return err
}

func (s *Store) ListTasks(ctx context.Context, ownerID string) ([]models.Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, owner_id, assignee_id, created_by_id, title, description, status, due_at, created_at, updated_at
		FROM tasks WHERE owner_id = $1 ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) TaskByID(ctx context.Context, id string) (*models.Task, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, owner_id, assignee_id, created_by_id, title, description, status, due_at, created_at, updated_at
		FROM tasks WHERE id = $1`, id)
	return scanTask(row)
}

func (s *Store) UpdateTask(ctx context.Context, t *models.Task) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE tasks SET title = $1, description = $2, status = $3, assignee_id = $4, due_at = $5, updated_at = $6
		WHERE id = $7 AND owner_id = $8`,
		t.Title, t.Description, t.Status, t.AssigneeID, timePtr(t.DueAt),
		t.UpdatedAt.UTC(), t.ID, t.OwnerID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTask(ctx context.Context, ownerID, taskID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1 AND owner_id = $2`, taskID, ownerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateReminder(ctx context.Context, r *models.Reminder) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO reminders (id, owner_id, title, body, due_at, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		r.ID, r.OwnerID, r.Title, r.Body, timePtr(r.DueAt), r.Status,
		r.CreatedAt.UTC(), r.UpdatedAt.UTC(),
	)
	return err
}

func (s *Store) ListReminders(ctx context.Context, ownerID string) ([]models.Reminder, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, owner_id, title, body, due_at, status, created_at, updated_at
		FROM reminders WHERE owner_id = $1 ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReminders(rows)
}

func (s *Store) ReminderByID(ctx context.Context, id string) (*models.Reminder, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, owner_id, title, body, due_at, status, created_at, updated_at
		FROM reminders WHERE id = $1`, id)
	return scanReminder(row)
}

func (s *Store) UpdateReminder(ctx context.Context, r *models.Reminder) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE reminders SET title = $1, body = $2, status = $3, due_at = $4, updated_at = $5
		WHERE id = $6 AND owner_id = $7`,
		r.Title, r.Body, r.Status, timePtr(r.DueAt),
		r.UpdatedAt.UTC(), r.ID, r.OwnerID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteReminder(ctx context.Context, ownerID, reminderID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM reminders WHERE id = $1 AND owner_id = $2`, reminderID, ownerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpsertTrustedContact(ctx context.Context, c *models.TrustedContact) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO trusted_contacts (owner_id, contact_user_id, can_assign_tasks, can_ping_reminders, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (owner_id, contact_user_id) DO UPDATE SET
			can_assign_tasks = EXCLUDED.can_assign_tasks,
			can_ping_reminders = EXCLUDED.can_ping_reminders`,
		c.OwnerID, c.ContactUserID, c.CanAssignTasks, c.CanPingReminders, c.CreatedAt.UTC(),
	)
	return err
}

func (s *Store) ListTrustedContacts(ctx context.Context, ownerID string) ([]models.TrustedContact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tc.owner_id, tc.contact_user_id, u.username, tc.can_assign_tasks, tc.can_ping_reminders, tc.created_at
		FROM trusted_contacts tc
		JOIN users u ON u.id = tc.contact_user_id
		WHERE tc.owner_id = $1
		ORDER BY tc.created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.TrustedContact
	for rows.Next() {
		var c models.TrustedContact
		if err := rows.Scan(&c.OwnerID, &c.ContactUserID, &c.ContactUsername, &c.CanAssignTasks, &c.CanPingReminders, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteTrustedContact(ctx context.Context, ownerID, contactUserID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM trusted_contacts WHERE owner_id = $1 AND contact_user_id = $2`,
		ownerID, contactUserID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) TrustedContact(ctx context.Context, ownerID, contactUserID string) (*models.TrustedContact, error) {
	var c models.TrustedContact
	err := s.db.QueryRowContext(ctx, `
		SELECT owner_id, contact_user_id, can_assign_tasks, can_ping_reminders, created_at
		FROM trusted_contacts WHERE owner_id = $1 AND contact_user_id = $2`,
		ownerID, contactUserID,
	).Scan(&c.OwnerID, &c.ContactUserID, &c.CanAssignTasks, &c.CanPingReminders, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateReminderPing(ctx context.Context, p *models.ReminderPing) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO reminder_pings (id, reminder_id, from_user_id, message, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		p.ID, p.ReminderID, p.FromUserID, p.Message, p.CreatedAt.UTC(),
	)
	return err
}

func (s *Store) ListReminderPings(ctx context.Context, ownerID string, limit int) ([]models.ReminderPing, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT rp.id, rp.reminder_id, rp.from_user_id, rp.message, rp.created_at
		FROM reminder_pings rp
		JOIN reminders r ON r.id = rp.reminder_id
		WHERE r.owner_id = $1
		ORDER BY rp.created_at DESC
		LIMIT $2`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ReminderPing
	for rows.Next() {
		var p models.ReminderPing
		if err := rows.Scan(&p.ID, &p.ReminderID, &p.FromUserID, &p.Message, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanTasks(rows *sql.Rows) ([]models.Task, error) {
	var out []models.Task
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func scanTask(row *sql.Row) (*models.Task, error) {
	t, err := scanTaskRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

func scanTaskRow(scanner interface{ Scan(dest ...any) error }) (*models.Task, error) {
	var t models.Task
	var assignee sql.NullString
	var due sql.NullTime
	err := scanner.Scan(&t.ID, &t.OwnerID, &assignee, &t.CreatedByID, &t.Title, &t.Description, &t.Status, &due, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if assignee.Valid {
		v := assignee.String
		t.AssigneeID = &v
	}
	if due.Valid {
		v := due.Time
		t.DueAt = &v
	}
	return &t, nil
}

func scanReminders(rows *sql.Rows) ([]models.Reminder, error) {
	var out []models.Reminder
	for rows.Next() {
		r, err := scanReminderRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func scanReminder(row *sql.Row) (*models.Reminder, error) {
	r, err := scanReminderRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return r, nil
}

func scanReminderRow(scanner interface{ Scan(dest ...any) error }) (*models.Reminder, error) {
	var r models.Reminder
	var due sql.NullTime
	err := scanner.Scan(&r.ID, &r.OwnerID, &r.Title, &r.Body, &due, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if due.Valid {
		v := due.Time
		r.DueAt = &v
	}
	return &r, nil
}

func timePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
