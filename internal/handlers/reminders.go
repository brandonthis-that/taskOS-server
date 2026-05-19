package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/brandonthis-that/taskOS-server/internal/models"
	"github.com/brandonthis-that/taskOS-server/internal/store"
)

type reminderRequest struct {
	Title  string  `json:"title"`
	Body   string  `json:"body"`
	Status string  `json:"status"`
	DueAt  *string `json:"due_at"`
}

func (a *API) ListReminders(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	reminders, err := a.store.ListReminders(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list reminders")
		return
	}
	if reminders == nil {
		reminders = []models.Reminder{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"reminders": reminders})
}

func (a *API) CreateReminder(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	var req reminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "title is required")
		return
	}
	dueAt, err := parseOptionalTime(req.DueAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "due_at must be RFC3339")
		return
	}
	status := req.Status
	if status == "" {
		status = models.ReminderStatusActive
	}
	if !validReminderStatus(status) {
		writeError(w, http.StatusBadRequest, "invalid_input", "invalid reminder status")
		return
	}

	now := time.Now().UTC()
	reminder := &models.Reminder{
		ID:        uuid.NewString(),
		OwnerID:   userID,
		Title:     req.Title,
		Body:      strings.TrimSpace(req.Body),
		DueAt:     dueAt,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := a.store.CreateReminder(r.Context(), reminder); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to create reminder")
		return
	}
	writeJSON(w, http.StatusCreated, reminder)
}

func (a *API) GetReminder(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	reminder, err := a.store.ReminderByID(r.Context(), chi.URLParam(r, "reminderID"))
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to load reminder")
		return
	}
	if reminder.OwnerID != userID {
		writeError(w, http.StatusNotFound, "not_found", "reminder not found")
		return
	}
	writeJSON(w, http.StatusOK, reminder)
}

func (a *API) UpdateReminder(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	reminder, err := a.store.ReminderByID(r.Context(), chi.URLParam(r, "reminderID"))
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to load reminder")
		return
	}
	if reminder.OwnerID != userID {
		writeError(w, http.StatusNotFound, "not_found", "reminder not found")
		return
	}

	var req reminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}
	if req.Title != "" {
		reminder.Title = strings.TrimSpace(req.Title)
	}
	if req.Body != "" || r.ContentLength > 0 {
		reminder.Body = strings.TrimSpace(req.Body)
	}
	if req.Status != "" {
		if !validReminderStatus(req.Status) {
			writeError(w, http.StatusBadRequest, "invalid_input", "invalid reminder status")
			return
		}
		reminder.Status = req.Status
	}
	if req.DueAt != nil {
		dueAt, err := parseOptionalTime(req.DueAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "due_at must be RFC3339")
			return
		}
		reminder.DueAt = dueAt
	}
	reminder.UpdatedAt = time.Now().UTC()

	if err := a.store.UpdateReminder(r.Context(), reminder); err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to update reminder")
		return
	}
	writeJSON(w, http.StatusOK, reminder)
}

func (a *API) DeleteReminder(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	if err := a.store.DeleteReminder(r.Context(), userID, chi.URLParam(r, "reminderID")); err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to delete reminder")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type pingRequest struct {
	Message string `json:"message"`
}

func (a *API) PingReminder(w http.ResponseWriter, r *http.Request) {
	fromUserID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	ownerID := chi.URLParam(r, "userID")
	reminderID := chi.URLParam(r, "reminderID")

	contact, err := a.store.TrustedContact(r.Context(), ownerID, fromUserID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusForbidden, "forbidden", "you are not a trusted contact of this user")
			return
		}
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "permission check failed")
		return
	}
	if !contact.CanPingReminders {
		writeError(w, http.StatusForbidden, "forbidden", "you do not have permission to ping reminders")
		return
	}

	reminder, err := a.store.ReminderByID(r.Context(), reminderID)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to load reminder")
		return
	}
	if reminder.OwnerID != ownerID {
		writeError(w, http.StatusNotFound, "not_found", "reminder not found")
		return
	}

	var req pingRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	ping := &models.ReminderPing{
		ID:         uuid.NewString(),
		ReminderID: reminderID,
		FromUserID: fromUserID,
		Message:    strings.TrimSpace(req.Message),
		CreatedAt:  time.Now().UTC(),
	}
	if err := a.store.CreateReminderPing(r.Context(), ping); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to record ping")
		return
	}
	writeJSON(w, http.StatusCreated, ping)
}

func (a *API) ListReminderPings(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	pings, err := a.store.ListReminderPings(r.Context(), userID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list pings")
		return
	}
	if pings == nil {
		pings = []models.ReminderPing{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"pings": pings})
}

func validReminderStatus(s string) bool {
	switch s {
	case models.ReminderStatusActive, models.ReminderStatusSnoozed, models.ReminderStatusDismissed:
		return true
	default:
		return false
	}
}
