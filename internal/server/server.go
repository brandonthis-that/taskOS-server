package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/brandonthis-that/taskOS-server/internal/auth"
	"github.com/brandonthis-that/taskOS-server/internal/config"
	"github.com/brandonthis-that/taskOS-server/internal/handlers"
	"github.com/brandonthis-that/taskOS-server/internal/middleware"
	"github.com/brandonthis-that/taskOS-server/internal/store"
)

// Server is the taskOS HTTP API.
type Server struct {
	cfg   config.Config
	http  *http.Server
	store *store.Store
}

// New builds and configures the HTTP server.
func New(cfg config.Config) (*Server, error) {
	st, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	authSvc := auth.New(st)
	api := handlers.NewAPI(st, authSvc)

	r := chi.NewRouter()
	r.Use(middleware.Recover)
	r.Use(middleware.Logger)
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(middleware.RateLimit(cfg.RateLimitPerMin))

	r.Get("/health", api.Health)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/register", api.Register)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(authSvc))

			r.Get("/me", api.Me)
			r.Get("/users/{username}", api.LookupUser)

			r.Get("/tasks", api.ListTasks)
			r.Post("/tasks", api.CreateTask)
			r.Get("/tasks/{taskID}", api.GetTask)
			r.Patch("/tasks/{taskID}", api.UpdateTask)
			r.Delete("/tasks/{taskID}", api.DeleteTask)

			r.Get("/reminders", api.ListReminders)
			r.Post("/reminders", api.CreateReminder)
			r.Get("/reminders/pings", api.ListReminderPings)
			r.Get("/reminders/{reminderID}", api.GetReminder)
			r.Patch("/reminders/{reminderID}", api.UpdateReminder)
			r.Delete("/reminders/{reminderID}", api.DeleteReminder)

			r.Get("/contacts", api.ListContacts)
			r.Post("/contacts", api.AddContact)
			r.Delete("/contacts/{contactUserID}", api.RemoveContact)

			r.Post("/users/{userID}/tasks", api.AssignTask)
			r.Post("/users/{userID}/reminders/{reminderID}/ping", api.PingReminder)
		})
	})

	srv := &Server{
		cfg:   cfg,
		store: st,
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           r,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
	return srv, nil
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	slog.Info("taskOS server listening", "addr", s.cfg.Addr)
	return s.http.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.http.Shutdown(ctx)
	if closeErr := s.store.Close(); closeErr != nil {
		return fmt.Errorf("shutdown http: %w, close db: %v", err, closeErr)
	}
	return err
}
