package auth

import (
	"context"
	"errors"
	"testing"
)

// fakeGate aceita um único código e conta usos e devoluções.
type fakeGate struct {
	valid    string
	consumed int
	released int
}

func (g *fakeGate) Consume(_ context.Context, code string) (func(context.Context), bool, error) {
	if code != g.valid {
		return nil, false, nil
	}
	g.consumed++
	return func(context.Context) { g.released++ }, true, nil
}

func TestModoConviteExigeCodigoValido(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	gate := &fakeGate{valid: "codigo-bom"}
	svc.RequireSignupCode(gate)

	for _, code := range []string{"", "errado", " codigo-bom"} {
		if _, _, err := svc.RegisterWithCode(ctx, "ana@exemplo.com", "Ana", "senha-forte-123", code); !errors.Is(err, ErrSignupCodeInvalid) {
			t.Errorf("código %q: esperava ErrSignupCodeInvalid, veio %v", code, err)
		}
	}
	// o Register simples (sem código) também é barrado no modo convite
	if _, _, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123"); !errors.Is(err, ErrSignupCodeInvalid) {
		t.Errorf("Register sem código deveria ser barrado, veio %v", err)
	}
	if gate.consumed != 0 {
		t.Errorf("nenhum uso deveria ser gasto, gastou %d", gate.consumed)
	}

	if _, _, err := svc.RegisterWithCode(ctx, "ana@exemplo.com", "Ana", "senha-forte-123", "codigo-bom"); err != nil {
		t.Fatalf("código válido deveria entrar: %v", err)
	}
	if gate.consumed != 1 || gate.released != 0 {
		t.Errorf("consumed=%d released=%d, esperado 1 e 0", gate.consumed, gate.released)
	}
}

// Dados inválidos não gastam o código, e se a criação falhar (e-mail repetido) o uso volta.
func TestModoConviteNaoGastaCodigoAtoa(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	gate := &fakeGate{valid: "codigo-bom"}
	svc.RequireSignupCode(gate)

	if _, _, err := svc.RegisterWithCode(ctx, "nao-e-email", "Ana", "senha-forte-123", "codigo-bom"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("esperava ErrInvalidInput, veio %v", err)
	}
	if gate.consumed != 0 {
		t.Errorf("dados inválidos gastaram o código (%d)", gate.consumed)
	}

	if _, _, err := svc.RegisterWithCode(ctx, "ana@exemplo.com", "Ana", "senha-forte-123", "codigo-bom"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RegisterWithCode(ctx, "ana@exemplo.com", "Outra", "senha-forte-123", "codigo-bom"); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("esperava ErrEmailTaken, veio %v", err)
	}
	if gate.consumed != 2 || gate.released != 1 {
		t.Errorf("consumed=%d released=%d, esperado 2 e 1 (o uso do cadastro que falhou volta)", gate.consumed, gate.released)
	}
}

func TestModoAbertoIgnoraOCodigo(t *testing.T) {
	svc := newTestService(t)
	if svc.InviteOnly() {
		t.Fatal("sem gate o cadastro é aberto")
	}
	if _, _, err := svc.RegisterWithCode(context.Background(), "ana@exemplo.com", "Ana", "senha-forte-123", "qualquer"); err != nil {
		t.Errorf("modo aberto deveria ignorar o código: %v", err)
	}
}
