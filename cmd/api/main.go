package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registra o driver "pgx" no database/sql

	"casal/internal/auth"
	"casal/internal/config"
	"casal/internal/household"
	"casal/internal/httpx"
	"casal/internal/push"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("falha fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "banco indisponível")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// contas e sessões
	authSvc, err := auth.NewService(auth.NewPostgresRepo(db), cfg.SessionTTL)
	if err != nil {
		return err
	}
	auth.NewHandler(authSvc, log, cfg.CookieSecure).Routes(mux)
	requireUser := auth.Require(authSvc)

	// casal e convite
	householdSvc := household.NewService(household.NewPostgresRepo(db))
	household.NewHandler(householdSvc, log).Routes(mux, requireUser)

	// push (antecipado: valida os avisos no celular antes de casal e lembretes)
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc := push.NewService(
			push.NewPostgresRepo(db),
			push.NewWebPushSender(cfg.VAPIDSubject, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey),
			log,
		)
		push.NewHandler(pushSvc, cfg.VAPIDPublicKey, log).Routes(mux, requireUser)
	} else {
		log.Warn("VAPID não configurado: rode go run ./cmd/vapid e preencha o .env")
	}

	// página de teste do PWA
	mux.Handle("GET /", http.FileServer(http.Dir("web")))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpx.Chain(mux, httpx.Recover(log), httpx.Logging(log)),
		ReadHeaderTimeout: 5 * time.Second,
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
