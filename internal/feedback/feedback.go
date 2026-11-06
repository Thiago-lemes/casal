// Package feedback recebe as mensagens que as pessoas escrevem para o time do app. É a única exceção a "nunca ver
// o conteúdo": a pessoa escreveu aquilo para nós, e a tela diz isso. O casal só escreve (não há rota para ler); quem
// lê é o painel de suporte. O texto não vai para log, métrica nem relatório de erros.
package feedback

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"casal/internal/apperr"
	"casal/internal/db"
	"casal/internal/event"
	"casal/internal/httpx"
	"casal/internal/identity"
)

var (
	ErrInvalidInput = apperr.New(apperr.Invalid, "dados inválidos")
	ErrRateLimited  = apperr.NewCoded(apperr.Conflict, "feedback_rate_limited", "limite de mensagens do dia atingido")
	ErrNotFound     = apperr.New(apperr.NotFound, "não encontrado")
)

const (
	maxMessage = 2000
	dailyLimit = 10
)

// Screens é a lista fechada de telas de onde a mensagem pode vir (a tabela tem um CHECK com a mesma lista).
var Screens = []string{"today", "tasks", "notes", "routine", "shopping", "reminders", "house", "finance", "goals", "profile", "other"}

// Item é uma mensagem, como o painel a lê.
type Item struct {
	ID          string
	HouseholdID string
	Author      string
	Message     string
	Screen      string
	CreatedAt   time.Time
	ReadAt      *time.Time
}

// Service guarda e lê as mensagens.
type Service struct {
	db     *sql.DB
	now    func() time.Time
	events event.Func
}

func NewService(conn *sql.DB) *Service { return &Service{db: conn, now: time.Now} }

// OnEvent registra quem guarda os eventos de produto (feedback_sent). Opcional.
func (s *Service) OnEvent(f event.Func) { s.events = f }

// Send guarda a mensagem de quem escreveu. No máximo 10 por pessoa em 24 horas.
func (s *Service) Send(ctx context.Context, householdID, userID, message, screen string) error {
	message = strings.TrimSpace(message)
	if n := utf8.RuneCountInString(message); n < 1 || n > maxMessage {
		return fmt.Errorf("%w: a mensagem tem de 1 a %d caracteres", ErrInvalidInput, maxMessage)
	}
	screen = strings.TrimSpace(screen)
	if screen != "" && !validScreen(screen) {
		return fmt.Errorf("%w: tela desconhecida", ErrInvalidInput)
	}
	now := s.now()
	err := db.InTx(ctx, s.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		// o lock serializa os envios da mesma pessoa: dois ao mesmo tempo não furam o limite
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "feedback:"+userID); err != nil {
			return err
		}
		var sent int
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM feedback WHERE user_id = $1 AND created_at > $2`,
			userID, now.Add(-24*time.Hour)).Scan(&sent); err != nil {
			return err
		}
		if sent >= dailyLimit {
			return ErrRateLimited
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO feedback (household_id, user_id, message, screen, created_at) VALUES ($1, $2, $3, $4, $5)`,
			householdID, userID, message, screen, now)
		return err
	})
	if err != nil {
		return err
	}
	s.events.Emit(ctx, event.FeedbackSent, userID, householdID)
	return nil
}

func validScreen(s string) bool {
	for _, v := range Screens {
		if v == s {
			return true
		}
	}
	return false
}

// List devolve as mensagens, as mais novas primeiro (até 200). Roda no escopo de sistema porque o painel atravessa
// casais; por isso só o painel o chama.
func (s *Service) List(ctx context.Context, onlyUnread bool) ([]Item, error) {
	out := []Item{}
	err := db.InTx(ctx, s.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT f.id, f.household_id, u.name, f.message, f.screen, f.created_at, f.read_at
			FROM feedback f JOIN users u ON u.id = f.user_id
			WHERE ($1 = false OR f.read_at IS NULL)
			ORDER BY f.created_at DESC LIMIT 200`, onlyUnread)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var it Item
			if err := rows.Scan(&it.ID, &it.HouseholdID, &it.Author, &it.Message, &it.Screen, &it.CreatedAt, &it.ReadAt); err != nil {
				return err
			}
			out = append(out, it)
		}
		return rows.Err()
	})
	return out, err
}

// MarkRead marca a mensagem como lida (idempotente).
func (s *Service) MarkRead(ctx context.Context, id string) error {
	return db.InTx(ctx, s.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE feedback SET read_at = coalesce(read_at, $2) WHERE id = $1`, id, s.now())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// Handler recebe o POST /v1/feedback. Não existe rota de leitura para o casal.
type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Routes(mux *http.ServeMux, requireUser, requireHousehold httpx.Middleware) {
	mux.Handle("POST /v1/feedback", httpx.Chain(http.HandlerFunc(h.send), requireUser, requireHousehold))
}

func (h *Handler) send(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Message string `json:"message"`
		Screen  string `json:"screen"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	if err := h.svc.Send(r.Context(), p.HouseholdID, p.UserID, in.Message, in.Screen); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
