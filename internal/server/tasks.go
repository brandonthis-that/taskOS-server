package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/brandonthis-that/taskOS-server/internal/store"
)

type createTaskReq struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueAt       *time.Time `json:"due_at,omitempty"`
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	u := userFromContext(r.Context())
	t, err := s.store.CreateTask(r.Context(), u.ID, req.Title, req.Description, req.DueAt)
	if err != nil {
		s.log.Error("create task", "err", err)
		writeError(w, http.StatusInternalServerError, "could not create task")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r.Context())
	tasks, err := s.store.ListTasks(r.Context(), u.ID)
	if err != nil {
		s.log.Error("list tasks", "err", err)
		writeError(w, http.StatusInternalServerError, "could not list tasks")
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r.Context())
	t, err := s.store.TaskByID(r.Context(), u.ID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		s.log.Error("get task", "err", err)
		writeError(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

type updateTaskReq struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	Done        *bool      `json:"done,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	ClearDueAt  bool       `json:"clear_due_at,omitempty"`
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	var req updateTaskReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u := userFromContext(r.Context())
	t, err := s.store.UpdateTask(r.Context(), u.ID, r.PathValue("id"),
		store.TaskUpdate{
			Title:       req.Title,
			Description: req.Description,
			Done:        req.Done,
			DueAt:       req.DueAt,
			ClearDueAt:  req.ClearDueAt,
		})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		s.log.Error("update task", "err", err)
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r.Context())
	if err := s.store.DeleteTask(r.Context(), u.ID, r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		s.log.Error("delete task", "err", err)
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
