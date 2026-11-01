package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"casal/internal/config"
	"casal/internal/db"
)

// Run sobe o servidor e os jobs, e só retorna quando o contexto é cancelado (desligamento gracioso).
func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	conn, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	if !cfg.CookieSecure {
		log.Warn("COOKIE_SECURE=false: use apenas em desenvolvimento local")
	}

	handler, jobs, err := New(cfg, conn, log)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		go job(ctx)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Info("servidor iniciado", "addr", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
