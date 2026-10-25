package push

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

type fakeRepo struct {
	subs    map[string][]Subscription // por usuário
	deleted []string
}

func (f *fakeRepo) Save(_ context.Context, userID string, sub Subscription, _ string) error {
	f.subs[userID] = append(f.subs[userID], sub)
	return nil
}

func (f *fakeRepo) Delete(context.Context, string, string) error { return nil }

func (f *fakeRepo) DeleteEndpoint(_ context.Context, endpoint string) error {
	f.deleted = append(f.deleted, endpoint)
	return nil
}

func (f *fakeRepo) ByUser(_ context.Context, userID string) ([]Subscription, error) {
	return f.subs[userID], nil
}

// fakeSender responde com o status configurado para cada endereço (200 se não houver).
type fakeSender struct {
	status map[string]int
	sent   []string
}

func (f *fakeSender) Send(_ context.Context, sub Subscription, _ []byte) (int, error) {
	f.sent = append(f.sent, sub.Endpoint)
	if s, ok := f.status[sub.Endpoint]; ok {
		return s, nil
	}
	return 201, nil
}

const (
	appleURL  = "https://web.push.apple.com/abc"
	googleURL = "https://fcm.googleapis.com/fcm/send/xyz"
)

func newTestService(subs map[string][]Subscription, status map[string]int) (*Service, *fakeRepo, *fakeSender) {
	repo := &fakeRepo{subs: subs}
	sender := &fakeSender{status: status}
	return NewService(repo, sender, slog.New(slog.DiscardHandler)), repo, sender
}

func TestNotifySendsToEveryDevice(t *testing.T) {
	svc, repo, sender := newTestService(map[string][]Subscription{
		"u1": {{Endpoint: appleURL}, {Endpoint: googleURL}},
	}, nil)

	if err := svc.Notify(context.Background(), "u1", "Oi", "teste"); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 2 {
		t.Errorf("deveria enviar para os 2 aparelhos, enviou para %d", len(sender.sent))
	}
	if len(repo.deleted) != 0 {
		t.Errorf("nada deveria ser apagado: %v", repo.deleted)
	}
}

func TestNotifyRemovesDeadSubscriptions(t *testing.T) {
	for _, status := range []int{404, 410} {
		svc, repo, _ := newTestService(map[string][]Subscription{
			"u1": {{Endpoint: appleURL}, {Endpoint: googleURL}},
		}, map[string]int{appleURL: status})

		if err := svc.Notify(context.Background(), "u1", "Oi", "teste"); err != nil {
			t.Errorf("status %d: uma assinatura morta não é erro do envio, veio %v", status, err)
		}
		if len(repo.deleted) != 1 || repo.deleted[0] != appleURL {
			t.Errorf("status %d: deveria apagar só o endereço morto, apagou %v", status, repo.deleted)
		}
	}
}

func TestNotifyReportsServiceErrors(t *testing.T) {
	svc, repo, sender := newTestService(map[string][]Subscription{
		"u1": {{Endpoint: appleURL}, {Endpoint: googleURL}},
	}, map[string]int{appleURL: 403})

	if err := svc.Notify(context.Background(), "u1", "Oi", "teste"); err == nil {
		t.Error("um 403 do serviço de push deveria virar erro")
	}
	if len(sender.sent) != 2 {
		t.Error("a falha em um aparelho não deve impedir o envio aos outros")
	}
	if len(repo.deleted) != 0 {
		t.Error("um 403 não significa assinatura morta")
	}
}

func TestNotifyWithoutDevices(t *testing.T) {
	svc, _, _ := newTestService(map[string][]Subscription{}, nil)
	if err := svc.Notify(context.Background(), "u1", "Oi", "teste"); !errors.Is(err, ErrNoDevices) {
		t.Errorf("esperava ErrNoDevices, veio %v", err)
	}
}

func TestSubscribeValidates(t *testing.T) {
	cases := []struct {
		name string
		sub  Subscription
	}{
		{"endereço desconhecido", Subscription{Endpoint: "https://evil.example.com/x", P256dh: "k", Auth: "a"}},
		{"sem chaves", Subscription{Endpoint: appleURL}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo, _ := newTestService(map[string][]Subscription{}, nil)
			if err := svc.Subscribe(context.Background(), "u1", c.sub, "ua"); !errors.Is(err, ErrInvalidSubscription) {
				t.Errorf("esperava ErrInvalidSubscription, veio %v", err)
			}
			if len(repo.subs["u1"]) != 0 {
				t.Error("nada deveria ter sido gravado")
			}
		})
	}

	svc, repo, _ := newTestService(map[string][]Subscription{}, nil)
	if err := svc.Subscribe(context.Background(), "u1", Subscription{Endpoint: appleURL, P256dh: "k", Auth: "a"}, "ua"); err != nil {
		t.Fatal(err)
	}
	if len(repo.subs["u1"]) != 1 {
		t.Error("a assinatura válida deveria ter sido gravada")
	}
}

func TestValidEndpoint(t *testing.T) {
	cases := map[string]bool{
		"https://web.push.apple.com/QGx":                      true,
		"https://fcm.googleapis.com/fcm/send/abc":             true,
		"https://updates.push.services.mozilla.com/wpush/v2/": true,
		"https://wns2-bl2p.notify.windows.com/w/?token=x":     true,
		"http://fcm.googleapis.com/fcm/send/abc":              false, // sem HTTPS
		"https://fcm.googleapis.com.evil.com/x":               false,
		"https://evilfcm.googleapis.com.example/x":            false,
		"https://localhost/x":                                 false,
		"nada":                                                false,
	}
	for url, want := range cases {
		if got := validEndpoint(url); got != want {
			t.Errorf("validEndpoint(%q) = %v, esperava %v", url, got, want)
		}
	}
}

// A biblioteca webpush-go (v1.4.0) acrescenta "mailto:" sozinha. Se o .env já tiver
// "mailto:", o token sairia com "mailto:mailto:" e a Apple recusaria (403 BadJwtToken).
func TestSubscriberWithoutMailtoPrefix(t *testing.T) {
	cases := map[string]string{
		"mailto:voce@exemplo.com": "voce@exemplo.com",
		"voce@exemplo.com":        "voce@exemplo.com",
		"https://casal.app":       "https://casal.app",
	}
	for in, want := range cases {
		if got := NewWebPushSender(in, "pub", "priv").subject; got != want {
			t.Errorf("subject de %q = %q, esperava %q", in, got, want)
		}
	}
}
