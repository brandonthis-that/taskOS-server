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

type taskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	DueAt       *string `json:"due_at"`
	AssigneeID  *string `json:"assignee_id"`
}

func (a *API) ListTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	tasks, err := a.store.ListTasks(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list tasks")
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (a *API) CreateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	var req taskRequest
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
		status = models.TaskStatusPending
	}
	if !validTaskStatus(status) {
		writeError(w, http.StatusBadRequest, "invalid_input", "invalid task status")
		return
	}

	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.NewString(),
		OwnerID:     userID,
		AssigneeID:  req.AssigneeID,
		CreatedByID: userID,
		Title:       req.Title,
		Description: strings.TrimSpace(req.Description),
		Status:      status,
		DueAt:       dueAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := a.store.CreateTask(r.Context(), task); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to create task")
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (a *API) GetTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	task, err := a.store.TaskByID(r.Context(), chi.URLParam(r, "taskID"))
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to load task")
		return
	}
	if task.OwnerID != userID {
		writeError(w, http.StatusNotFound, "not_found", "task not found")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (a *API) UpdateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	task, err := a.store.TaskByID(r.Context(), chi.URLParam(r, "taskID"))
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to load task")
		return
	}
	if task.OwnerID != userID {
		writeError(w, http.StatusNotFound, "not_found", "task not found")
		return
	}

	var req taskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}
	if req.Title != "" {
		task.Title = strings.TrimSpace(req.Title)
	}
	if req.Description != "" || r.ContentLength > 0 {
		task.Description = strings.TrimSpace(req.Description)
	}
	if req.Status != "" {
		if !validTaskStatus(req.Status) {
			writeError(w, http.StatusBadRequest, "invalid_input", "invalid task status")
			return
		}
		task.Status = req.Status
	}
	if req.DueAt != nil {
		dueAt, err := parseOptionalTime(req.DueAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "due_at must be RFC3339")
			return
		}
		task.DueAt = dueAt
	}
	if req.AssigneeID != nil {
		task.AssigneeID = req.AssigneeID
	}
	task.UpdatedAt = time.Now().UTC()

	if err := a.store.UpdateTask(r.Context(), task); err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to update task")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (a *API) DeleteTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	if err := a.store.DeleteTask(r.Context(), userID, chi.URLParam(r, "taskID")); err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to delete task")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type assignTaskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	DueAt       *string `json:"due_at"`
}

func (a *API) AssignTask(w http.ResponseWriter, r *http.Request) {
	fromUserID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	ownerID := chi.URLParam(r, "userID")
	if ownerID == fromUserID {
		writeError(w, http.StatusBadRequest, "invalid_input", "use POST /v1/tasks to create your own tasks")
		return
	}

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
	if !contact.CanAssignTasks {
		writeError(w, http.StatusForbidden, "forbidden", "you do not have permission to assign tasks")
		return
	}

	var req assignTaskRequest
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

	now := time.Now().UTC()
	assignee := fromUserID
	task := &models.Task{
		ID:          uuid.NewString(),
		OwnerID:     ownerID,
		AssigneeID:  &assignee,
		CreatedByID: fromUserID,
		Title:       req.Title,
		Description: strings.TrimSpace(req.Description),
		Status:      models.TaskStatusPending,
		DueAt:       dueAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := a.store.CreateTask(r.Context(), task); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to assign task")
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func validTaskStatus(s string) bool {
	switch s {
	case models.TaskStatusPending, models.TaskStatusInProgress, models.TaskStatusDone, models.TaskStatusCancelled:
		return true
	default:
		return false
	}
}

func parseOptionalTime(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	utc := t.UTC()
	return &utc, nil
}
