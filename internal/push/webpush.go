package push

import (
	"context"
	"strings"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type WebPushSender struct {
	subject, publicKey, privateKey string
}

// NewWebPushSender recebe o VAPID_SUBJECT com ou sem "mailto:". A webpush-go v1.4.0
// acrescenta "mailto:" a tudo que não começa com "https:", então tiramos o prefixo
// aqui para o token não sair com "mailto:mailto:", que a Apple recusa.
func NewWebPushSender(subject, publicKey, privateKey string) *WebPushSender {
	return &WebPushSender{
		subject:    strings.TrimPrefix(subject, "mailto:"),
		publicKey:  publicKey,
		privateKey: privateKey,
	}
}

func (w *WebPushSender) Send(ctx context.Context, sub Subscription, payload []byte) (int, error) {
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{Auth: sub.Auth, P256dh: sub.P256dh},
	}, &webpush.Options{
		Subscriber:      w.subject,
		VAPIDPublicKey:  w.publicKey,
		VAPIDPrivateKey: w.privateKey,
		TTL:             3600,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
