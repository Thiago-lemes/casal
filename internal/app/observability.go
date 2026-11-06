package app

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"casal/internal/config"
	"casal/internal/ops"
	"casal/internal/push"
	"casal/internal/reminder"
)

// overdueGrace é a folga antes de um lembrete contar como atrasado: o agendador roda a cada 15 s, então um
// lembrete que passou da hora há mais de um minuto sem sair indica fila travada, não demora normal.
const overdueGrace = time.Minute

// observability reúne as métricas e o relatório de erros. Os rótulos são só os da spec (docs/spec-fase-5-operacao.md):
// nunca id de pessoa ou de casal, e nunca o caminho real.
type observability struct {
	reg      *ops.Registry
	reporter *ops.Reporter
	http     *ops.HTTPMetrics

	reminderSend *ops.CounterVec
	pushSend     *ops.CounterVec
	lastTick     *ops.GaugeVec
	maintenance  *ops.GaugeVec
}

func newObservability(cfg config.Config, conn *sql.DB, log *slog.Logger) (*observability, error) {
	reg := ops.NewRegistry()
	o := &observability{
		reg:          reg,
		http:         ops.NewHTTPMetrics(reg),
		reminderSend: reg.Counter("reminder_send_total", "Lembretes enviados, por resultado.", "result"),
		pushSend:     reg.Counter("push_send_total", "Envios de push, por status e serviço.", "status", "host"),
		lastTick:     reg.Gauge("reminder_worker_last_tick_timestamp", "Hora do último ciclo do agendador."),
		maintenance:  reg.Gauge("maintenance_last_success_timestamp", "Hora da última rodada sem falha, por job.", "job"),
	}
	reg.OnScrape(func(w io.Writer) {
		ops.WriteGauge(w, "db_connections_in_use", "Conexões do banco em uso.", float64(conn.Stats().InUse))
	})

	rp, err := ops.NewReporter(ops.ReporterOptions{DSN: cfg.SentryDSN, Environment: cfg.Environment, Release: cfg.Release})
	if err != nil {
		return nil, err
	}
	o.reporter = rp
	if rp != nil {
		log.Info("relatório de erros ligado")
	}
	return o, nil
}

// watchQueue registra a medida de lembretes atrasados, lida na hora em que o /metrics é consultado.
func (o *observability) watchQueue(repo *reminder.PostgresRepo, log *slog.Logger) {
	o.reg.OnScrape(func(w io.Writer) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		n, err := repo.CountOverdue(ctx, time.Now(), overdueGrace)
		if err != nil {
			log.Warn("não consegui contar os lembretes atrasados", "err", err)
			return // sem a medida é melhor que uma medida falsa
		}
		ops.WriteGauge(w, "reminder_queue_due", "Lembretes agendados que já passaram da hora há mais de 1 minuto.", float64(n))
	})
}

func (o *observability) markMaintenance(job string) { o.maintenance.SetNow(job) }

// observePush conta cada envio de push. Códigos fora da lista viram classes para não multiplicar as séries.
func (o *observability) observePush(status int, host string, err error) {
	label := "error"
	switch {
	case err != nil:
	case status >= 200 && status < 300:
		label = "200"
	case status == 403 || status == 404 || status == 410:
		label = strconv.Itoa(status)
	case status >= 400 && status < 500:
		label = "4xx"
	case status >= 500:
		label = "5xx"
	}
	o.pushSend.Inc(label, host)
}

// meter conta o resultado de cada lembrete entregue pelo agendador: ok, no_devices (a pessoa não ativou avisos) ou error.
func (o *observability) meter(n reminder.Notifier) reminder.Notifier {
	return meteredNotifier{inner: n, count: o.reminderSend}
}

type meteredNotifier struct {
	inner reminder.Notifier
	count *ops.CounterVec
}

func (m meteredNotifier) Notify(ctx context.Context, userID, title, body string) error {
	err := m.inner.Notify(ctx, userID, title, body)
	switch {
	case err == nil:
		m.count.Inc("ok")
	case errors.Is(err, push.ErrNoDevices):
		m.count.Inc("no_devices")
	default:
		m.count.Inc("error")
	}
	return err
}

// metricsJob serve o /metrics num endereço interno separado, fora do servidor público.
func (o *observability) metricsJob(addr, token string, log *slog.Logger) Job {
	return func(ctx context.Context) {
		var h http.Handler = o.reg.Handler()
		if token != "" {
			h = ops.RequireBearer(token, h)
		}
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", h)
		srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
		}()
		log.Info("métricas em endereço interno", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("servidor de métricas parou", "err", err)
		}
	}
}
