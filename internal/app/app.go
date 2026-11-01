// Package app monta a aplicação: liga as features, as rotas e os processos em segundo plano.
// É o único lugar que conhece todas as features.
package app

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"casal/internal/auth"
	"casal/internal/config"
	"casal/internal/household"
	"casal/internal/httpx"
	"casal/internal/push"
	"casal/internal/reminder"
)

// Job é um processo em segundo plano; deve rodar até o contexto ser cancelado.
type Job func(ctx context.Context)

// New liga as features e devolve o handler HTTP e os jobs. Não abre portas nem inicia nada.
func New(cfg config.Config, conn *sql.DB, log *slog.Logger) (http.Handler, []Job, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := conn.PingContext(r.Context()); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "banco indisponível")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// contas e sessões
	authSvc, err := auth.NewService(auth.NewPostgresRepo(conn), cfg.SessionTTL)
	if err != nil {
		return nil, nil, err
	}
	auth.NewHandler(authSvc, log, auth.Options{CookieSecure: cfg.CookieSecure, TrustProxy: cfg.TrustProxy}).Routes(mux)
	requireUser := auth.Require(authSvc)

	// casal e convite
	householdSvc := household.NewService(household.NewPostgresRepo(conn))
	household.NewHandler(householdSvc, log).Routes(mux, requireUser)
	requireHousehold := household.RequireHousehold(householdSvc, log)

	// push, com o notificador de log como alternativa quando o VAPID não está configurado
	var notifier reminder.Notifier = logNotifier{log}
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc := push.NewService(
			push.NewPostgresRepo(conn),
			push.NewWebPushSender(cfg.VAPIDSubject, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey),
			log,
		)
		push.NewHandler(pushSvc, cfg.VAPIDPublicKey, log).Routes(mux, requireUser)
		notifier = pushSvc
	} else {
		log.Warn("VAPID não configurado: avisos só no log. Rode go run ./cmd/vapid e preencha o .env")
	}

	// lembretes
	reminderRepo := reminder.NewPostgresRepo(conn)
	reminder.NewHandler(reminder.NewService(reminderRepo, householdSvc), log).
		Routes(mux, requireUser, requireHousehold)

	// página de teste do PWA
	mux.Handle("GET /", http.FileServer(http.Dir("web")))

	handler := httpx.Chain(mux,
		httpx.RequestID(), // o mais externo: todo log abaixo carrega o ID
		httpx.Recover(log),
		httpx.Logging(log),
		httpx.SecurityHeaders(cfg.CookieSecure), // CookieSecure indica que o site é servido por HTTPS
		httpx.Timeout(15*time.Second),
	)
	jobs := []Job{
		reminder.NewWorker(reminderRepo, notifier, log).Run,
		maintenance(log, authSvc.PurgeExpired, householdSvc.PurgeStaleInvites),
	}
	return handler, jobs, nil
}

// logNotifier só escreve no log; usado enquanto o push não está configurado.
type logNotifier struct{ log *slog.Logger }

func (n logNotifier) Notify(_ context.Context, userID, title, body string) error {
	n.log.Info("notificação (simulada)", "user", userID, "title", title, "body", body)
	return nil
}
