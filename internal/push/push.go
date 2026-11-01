package push

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"casal/internal/apperr"
)

var (
	ErrNoDevices           = apperr.New(apperr.Conflict, "nenhum aparelho com avisos ativos")
	ErrInvalidSubscription = apperr.New(apperr.Invalid, "assinatura inválida")
	// ErrDelivery é o que o cliente vê quando o serviço de push recusa ou falha; o detalhe vai só para o log.
	ErrDelivery = apperr.New(apperr.Upstream, "não consegui enviar o aviso")
)

type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Sender isola a biblioteca de Web Push. Em testes, use um fake.
type Sender interface {
	Send(ctx context.Context, sub Subscription, payload []byte) (status int, err error)
}

type Repository interface {
	Save(ctx context.Context, userID string, sub Subscription, userAgent string) error
	Delete(ctx context.Context, userID, endpoint string) error
	DeleteEndpoint(ctx context.Context, endpoint string) error
	ByUser(ctx context.Context, userID string) ([]Subscription, error)
}

type Service struct {
	repo   Repository
	sender Sender
	log    *slog.Logger
}

func NewService(repo Repository, sender Sender, log *slog.Logger) *Service {
	return &Service{repo: repo, sender: sender, log: log}
}

// Subscribe guarda o aparelho da pessoa, aceitando só endereços de serviços de push conhecidos.
func (s *Service) Subscribe(ctx context.Context, userID string, sub Subscription, userAgent string) error {
	if !validEndpoint(sub.Endpoint) || sub.P256dh == "" || sub.Auth == "" {
		return ErrInvalidSubscription
	}
	return s.repo.Save(ctx, userID, sub, userAgent)
}

func (s *Service) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	return s.repo.Delete(ctx, userID, endpoint)
}

// Notify avisa todos os aparelhos da pessoa. Implementa reminder.Notifier.
func (s *Service) Notify(ctx context.Context, userID, title, body string) error {
	subs, err := s.repo.ByUser(ctx, userID)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return ErrNoDevices
	}
	payload, err := json.Marshal(map[string]string{"title": title, "body": body, "url": "/"})
	if err != nil {
		return err
	}

	var firstErr error
	for _, sub := range subs {
		status, err := s.sender.Send(ctx, sub, payload)
		switch {
		case status == 404 || status == 410:
			// o aparelho cancelou ou a assinatura expirou: apague para não tentar de novo
			s.log.Info("assinatura de push removida", "status", status, "host", hostOf(sub.Endpoint))
			_ = s.repo.DeleteEndpoint(ctx, sub.Endpoint)
		case err != nil:
			if firstErr == nil {
				firstErr = fmt.Errorf("%w: %v", ErrDelivery, err)
			}
		case status >= 400:
			// 403 da Apple costuma ser problema no token VAPID (por exemplo, o "sub")
			s.log.Warn("serviço de push recusou", "status", status, "host", hostOf(sub.Endpoint))
			if firstErr == nil {
				firstErr = fmt.Errorf("%w: serviço de push respondeu %d", ErrDelivery, status)
			}
		}
	}
	return firstErr
}

// Os endereços de push vêm do navegador, então o servidor só aceita os dos serviços
// conhecidos. Sem isto, alguém poderia fazer o servidor chamar qualquer endereço (SSRF).
var allowedHosts = []string{
	"fcm.googleapis.com",
	"push.services.mozilla.com",
	"web.push.apple.com",
	"notify.windows.com",
}

func validEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range allowedHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

// hostOf serve para logar o serviço sem expor o endereço completo do aparelho.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
