// Package server exposes the taskOS HTTP API.
//
// Routes are mounted via stdlib http.ServeMux (Go 1.22+ method-aware
// patterns); no external router is needed.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/brandonthis-that/taskOS-server/internal/store"
)

type Server struct {
	store      *store.Store
	log        *slog.Logger
	sessionDur time.Duration
}

func New(s *store.Store, log *slog.Logger, sessionDur time.Duration) *Server {
	return &Server{store: s, log: log, sessionDur: sessionDur}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.healthz)

	mux.HandleFunc("POST /api/auth/signup", s.signup)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.Handle("POST /api/auth/logout", s.requireAuth(http.HandlerFunc(s.logout)))
	mux.Handle("GET /api/auth/me", s.requireAuth(http.HandlerFunc(s.me)))

	mux.Handle("GET /api/tasks", s.requireAuth(http.HandlerFunc(s.listTasks)))
	mux.Handle("POST /api/tasks", s.requireAuth(http.HandlerFunc(s.createTask)))
	mux.Handle("GET /api/tasks/{id}", s.requireAuth(http.HandlerFunc(s.getTask)))
	mux.Handle("PATCH /api/tasks/{id}", s.requireAuth(http.HandlerFunc(s.updateTask)))
	mux.Handle("DELETE /api/tasks/{id}", s.requireAuth(http.HandlerFunc(s.deleteTask)))

	return s.withLogging(mux)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Pool.Ping(r.Context()); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
