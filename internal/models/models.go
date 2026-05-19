package models

import "time"

const (
	TaskStatusPending    = "pending"
	TaskStatusInProgress = "in_progress"
	TaskStatusDone       = "done"
	TaskStatusCancelled  = "cancelled"

	ReminderStatusActive    = "active"
	ReminderStatusSnoozed   = "snoozed"
	ReminderStatusDismissed = "dismissed"
)

// User is an account that owns tasks and reminders.
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// Task belongs to an owner and may be assigned by a trusted contact.
type Task struct {
	ID          string     `json:"id"`
	OwnerID     string     `json:"owner_id"`
	AssigneeID  *string    `json:"assignee_id,omitempty"`
	CreatedByID string     `json:"created_by_id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Reminder is a time-based nudge owned by a user.
type Reminder struct {
	ID        string     `json:"id"`
	OwnerID   string     `json:"owner_id"`
	Title     string     `json:"title"`
	Body      string     `json:"body,omitempty"`
	DueAt     *time.Time `json:"due_at,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TrustedContact grants another user limited actions on your account.
type TrustedContact struct {
	OwnerID          string    `json:"owner_id"`
	ContactUserID    string    `json:"contact_user_id"`
	ContactUsername  string    `json:"contact_username,omitempty"`
	CanAssignTasks   bool      `json:"can_assign_tasks"`
	CanPingReminders bool      `json:"can_ping_reminders"`
	CreatedAt        time.Time `json:"created_at"`
}

// ReminderPing records that someone nudged a reminder.
type ReminderPing struct {
	ID         string    `json:"id"`
	ReminderID string    `json:"reminder_id"`
	FromUserID string    `json:"from_user_id"`
	Message    string    `json:"message,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// APIError is the standard error envelope returned to clients.
type APIError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}
