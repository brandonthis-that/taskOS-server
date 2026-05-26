package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brandonthis-that/taskOS-server/internal/config"
	"github.com/brandonthis-that/taskOS-server/internal/server"
	"github.com/brandonthis-that/taskOS-server/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	connectCtx, ccancel := context.WithTimeout(ctx, 10*time.Second)
	st, err := store.New(connectCtx, cfg.DB.DSN())
	ccancel()
	if err != nil {
		return err
	}
	defer st.Close()

	srv := server.New(st, log, time.Duration(cfg.SessionHours)*time.Hour)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-serverErr:
		if err != nil {
			return err
		}
	}

	shutdownCtx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer scancel()
	return httpSrv.Shutdown(shutdownCtx)
}
