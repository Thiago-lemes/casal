// Package app monta a aplicação: liga as features, as rotas e os processos em segundo plano.
// É o único lugar que conhece todas as features.
package app

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"casal/internal/admin"
	"casal/internal/auth"
	"casal/internal/config"
	"casal/internal/feedback"
	"casal/internal/finance"
	"casal/internal/goal"
	"casal/internal/household"
	"casal/internal/httpx"
	"casal/internal/module"
	"casal/internal/note"
	"casal/internal/ops"
	"casal/internal/push"
	"casal/internal/reminder"
	"casal/internal/routine"
	"casal/internal/shopping"
	"casal/internal/signup"
	"casal/internal/task"
)

// Job é um processo em segundo plano; deve rodar até o contexto ser cancelado.
type Job func(ctx context.Context)

// New liga as features e devolve o handler HTTP e os jobs. Não abre portas nem inicia nada.
func New(cfg config.Config, conn *sql.DB, log *slog.Logger) (http.Handler, []Job, error) {
	obs, err := newObservability(cfg, conn, log)
	if err != nil {
		return nil, nil, err
	}
	if obs.reporter != nil {
		httpx.OnServerError(obs.reporter.Hook())
	}

	mux := http.NewServeMux()
	// com METRICS_TOKEN, o /metrics também sai na porta principal, exigindo o token; sem ele, só no METRICS_ADDR
	if cfg.MetricsToken != "" {
		mux.Handle("GET /metrics", ops.RequireBearer(cfg.MetricsToken, obs.reg.Handler()))
	}
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
	events := ops.NewEvents(conn, log)
	authSvc.OnEvent(events.Record)
	signupSvc := signup.NewService(conn)
	if cfg.SignupMode == "invite" {
		authSvc.RequireSignupCode(signupSvc)
		log.Info("cadastro por convite ligado (SIGNUP_MODE=invite)")
	}
	auth.NewHandler(authSvc, log, auth.Options{CookieSecure: cfg.CookieSecure, TrustProxy: cfg.TrustProxy}).Routes(mux)
	requireUser := auth.Require(authSvc)

	// casal e convite
	householdSvc := household.NewService(household.NewPostgresRepo(conn))
	householdSvc.OnEvent(events.Record)
	household.NewHandler(householdSvc, log).Routes(mux, requireUser)
	requireHousehold := household.RequireHousehold(householdSvc, log)

	// módulos opcionais do casal; as features de cada módulo usam module.Require(moduleSvc, log, nome) nas rotas
	moduleSvc := module.NewService(module.NewPostgresRepo(conn))
	moduleSvc.OnEvent(events.Record)
	module.NewHandler(moduleSvc, log).Routes(mux, requireUser, requireHousehold)

	// push, com o notificador de log como alternativa quando o VAPID não está configurado
	var notifier reminder.Notifier = logNotifier{log}
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc := push.NewService(
			push.NewPostgresRepo(conn),
			push.NewWebPushSender(cfg.VAPIDSubject, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey),
			log,
		)
		pushSvc.Observe(obs.observePush)
		pushSvc.OnEvent(events.Record)
		push.NewHandler(pushSvc, cfg.VAPIDPublicKey, log).Routes(mux, requireUser)
		notifier = pushSvc
	} else {
		log.Warn("VAPID não configurado: avisos só no log. Rode go run ./cmd/vapid e preencha o .env")
	}

	// lembretes
	reminderRepo := reminder.NewPostgresRepo(conn)
	obs.watchQueue(reminderRepo, log)
	reminderSvc := reminder.NewService(reminderRepo, householdSvc, notifier, log)
	reminderSvc.OnEvent(events.Record)
	reminder.NewHandler(reminderSvc, log).Routes(mux, requireUser, requireHousehold)

	// tarefas
	taskRepo := task.NewPostgresRepo(conn)
	taskSvc := task.NewService(taskRepo, householdSvc, notifier, log)
	task.NewHandler(taskSvc, log).Routes(mux, requireUser, requireHousehold)

	// rotina semanal
	routineRepo := routine.NewPostgresRepo(conn)
	routine.NewHandler(routine.NewService(routineRepo, routinePeople{householdSvc}), log).
		Routes(mux, requireUser, requireHousehold)

	// lista de compras
	shoppingSvc := shopping.NewService(shopping.NewPostgresRepo(conn))
	shopping.NewHandler(shoppingSvc, log).Routes(mux, requireUser, requireHousehold)

	// anotações
	noteRepo := note.NewPostgresRepo(conn)
	note.NewHandler(note.NewService(noteRepo), log).
		Routes(mux, requireUser, requireHousehold)

	// finanças (módulo opcional): as rotas exigem o módulo ligado; as categorias padrão nascem ao ligá-lo e os
	// lembretes das contas acompanham o módulo
	financeRepo := finance.NewPostgresRepo(conn)
	financeSvc := finance.NewService(financeRepo, financePeople{householdSvc}, financeReminders{reminderSvc}, notifier, log)
	finance.NewHandler(financeSvc, log).
		Routes(mux, requireUser, requireHousehold, module.Require(moduleSvc, log, module.Finance))
	moduleSvc.OnEnable(module.Finance, finance.SeedCategories)
	moduleSvc.AfterChange(module.Finance, financeSvc.ModuleChanged)

	// metas (módulo opcional): progresso calculado no servidor; as etapas de projeto são tarefas (ligadas por ref) e o
	// hábito pode ter lembrete. Quando uma tarefa com ref de meta é concluída ou cancelada, a meta é recalculada.
	goalRepo := goal.NewPostgresRepo(conn)
	goalSvc := goal.NewService(goalRepo, goalTasks{taskSvc}, goalReminders{reminderSvc}, goalPeople{householdSvc},
		notifier, goalSpending{financeSvc}, moduleSvc, log)
	goal.NewHandler(goalSvc, log).
		Routes(mux, requireUser, requireHousehold, module.Require(moduleSvc, log, module.Goals))
	moduleSvc.AfterChange(module.Goals, goalSvc.ModuleChanged)
	taskSvc.AfterChange(func(ctx context.Context, t task.Task) {
		if t.Ref != nil {
			goalSvc.TaskChanged(ctx, t.HouseholdID, *t.Ref, t.Status)
		}
	})

	// feedback: o casal só escreve; quem lê é o painel de suporte
	feedbackSvc := feedback.NewService(conn)
	feedbackSvc.OnEvent(events.Record)
	feedback.NewHandler(feedbackSvc, log).Routes(mux, requireUser, requireHousehold)

	// painel de suporte: ligado só com ADMIN_ENABLED=true; sem isso, tudo em /admin/v1 responde 404
	adminCleanup := func(context.Context) (int64, error) { return 0, nil }
	if cfg.AdminEnabled {
		adminSvc, err := admin.NewService(admin.NewPostgresRepo(conn), admin.Deps{
			Directory:   adminDirectory{ops.NewInspector(conn)},
			Modules:     adminModules{moduleSvc},
			Users:       authSvc,
			SignupCodes: adminCodes{signupSvc},
			Feedback:    adminFeedback{feedbackSvc},
		})
		if err != nil {
			return nil, nil, err
		}
		admin.NewHandler(adminSvc, log, admin.Options{CookieSecure: cfg.CookieSecure, TrustProxy: cfg.TrustProxy}).Routes(mux)
		adminCleanup = adminSvc.PurgeExpired
		log.Warn("painel de suporte ligado (ADMIN_ENABLED=true)")
	} else {
		// um padrão por método: "/admin/v1/" sem método conflitaria com o "GET /" da página de teste
		for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
			mux.Handle(method+" /admin/v1/", admin.Disabled())
		}
	}

	// mudança de casal: cada feature leva o que é pessoal e passa adiante o que era compartilhado,
	// na mesma transação. Registrados aqui, antes de atender qualquer requisição.
	householdSvc.OnLeave(noteRepo.HandleLeave)
	householdSvc.OnLeave(routineRepo.HandleLeave)
	householdSvc.OnLeave(taskRepo.HandleLeave)
	householdSvc.OnLeave(reminderRepo.HandleLeave)
	householdSvc.OnLeave(financeRepo.HandleLeave)
	householdSvc.OnLeave(goalRepo.HandleLeave)
	householdSvc.OnJoin(noteRepo.HandleJoin)
	householdSvc.OnJoin(routineRepo.HandleJoin)
	householdSvc.OnJoin(reminderRepo.HandleJoin)
	householdSvc.OnJoin(financeRepo.HandleJoin)
	householdSvc.OnJoin(goalRepo.HandleJoin)

	// página de teste do PWA
	mux.Handle("GET /", http.FileServer(http.Dir("web")))

	reminderWorker := reminder.NewWorker(reminderRepo, obs.meter(notifier), log)
	reminderWorker.OnTick(func() { obs.lastTick.SetNow() })

	handler := httpx.Chain(obs.http.Wrap(mux),
		httpx.RequestID(), // o mais externo: todo log abaixo carrega o ID
		httpx.RouteHolder(),
		httpx.Recover(log),
		httpx.Logging(log),
		httpx.SecurityHeaders(cfg.CookieSecure), // CookieSecure indica que o site é servido por HTTPS
		httpx.Timeout(15*time.Second),
	)
	jobs := []Job{
		reminderWorker.Run,
		maintenance(log, "limpeza", obs.markMaintenance, authSvc.PurgeExpired, householdSvc.PurgeStaleInvites, shoppingSvc.PurgeBought, events.Purge, adminCleanup),
		maintenance(log, "financas", obs.markMaintenance, financeSvc.GenerateRecurring, financeSvc.ReconcileReminders),
		maintenance(log, "metas", obs.markMaintenance, goalSvc.ReconcileHabitReminders, goalSvc.ReconcileSteps),
	}
	if cfg.MetricsAddr != "" {
		jobs = append(jobs, obs.metricsJob(cfg.MetricsAddr, cfg.MetricsToken, log))
	}
	return handler, jobs, nil
}

// logNotifier só escreve no log; usado enquanto o push não está configurado.
type logNotifier struct{ log *slog.Logger }

func (n logNotifier) Notify(_ context.Context, userID, title, body string) error {
	n.log.Info("notificação (simulada)", "user", userID, "title", title, "body", body)
	return nil
}
