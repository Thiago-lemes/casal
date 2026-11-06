package signup

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"casal/internal/testutil"
)

func newCode(t *testing.T, s *Service, uses int) (Code, string) {
	t.Helper()
	c, raw, err := s.Create(context.Background(), "teste "+t.Name(), uses, 24*time.Hour, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.db.Exec(`DELETE FROM signup_codes WHERE id = $1`, c.ID) })
	return c, raw
}

func TestConsumeAceitaCodigoValidoEGastaUmUso(t *testing.T) {
	s := NewService(testutil.OpenDB(t))
	c, raw := newCode(t, s, 2)
	ctx := context.Background()

	for i := 1; i <= 2; i++ {
		if _, ok, err := s.Consume(ctx, raw); err != nil || !ok {
			t.Fatalf("uso %d: ok=%v err=%v", i, ok, err)
		}
	}
	if _, ok, _ := s.Consume(ctx, raw); ok {
		t.Error("o terceiro uso de um código de 2 não pode passar")
	}
	var uses int
	_ = s.db.QueryRow(`SELECT uses FROM signup_codes WHERE id = $1`, c.ID).Scan(&uses)
	if uses != 2 {
		t.Errorf("uses = %d, esperado 2", uses)
	}
}

// Sem revelar a causa: código errado, vazio, vencido e revogado são todos "não vale".
func TestConsumeRecusaCodigosQueNaoValem(t *testing.T) {
	s := NewService(testutil.OpenDB(t))
	ctx := context.Background()

	if _, ok, err := s.Consume(ctx, ""); ok || err != nil {
		t.Errorf("vazio: ok=%v err=%v", ok, err)
	}
	if _, ok, err := s.Consume(ctx, "codigo-que-nao-existe"); ok || err != nil {
		t.Errorf("inexistente: ok=%v err=%v", ok, err)
	}

	expired, rawExpired := newCode(t, s, 1)
	_, _ = s.db.Exec(`UPDATE signup_codes SET expires_at = now() - interval '1 minute' WHERE id = $1`, expired.ID)
	if _, ok, _ := s.Consume(ctx, rawExpired); ok {
		t.Error("código vencido passou")
	}

	revoked, rawRevoked := newCode(t, s, 1)
	if err := s.Revoke(ctx, revoked.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Consume(ctx, rawRevoked); ok {
		t.Error("código revogado passou")
	}
}

// Dois cadastros ao mesmo tempo com o último uso: só um passa.
func TestConsumeUltimoUsoEAtomico(t *testing.T) {
	s := NewService(testutil.OpenDB(t))
	_, raw := newCode(t, s, 1)

	var passed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok, err := s.Consume(context.Background(), raw); err == nil && ok {
				passed.Add(1)
			}
		}()
	}
	wg.Wait()
	if passed.Load() != 1 {
		t.Errorf("%d cadastros passaram com um código de 1 uso", passed.Load())
	}
}

func TestReleaseDevolveOUso(t *testing.T) {
	s := NewService(testutil.OpenDB(t))
	_, raw := newCode(t, s, 1)
	ctx := context.Background()

	release, ok, err := s.Consume(ctx, raw)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	release(ctx)
	if _, ok, _ := s.Consume(ctx, raw); !ok {
		t.Error("depois do release o código deve voltar a valer")
	}
}

func TestCreateValidaOsLimites(t *testing.T) {
	s := NewService(testutil.OpenDB(t))
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"usos zero":      func() error { _, _, err := s.Create(ctx, "x", 0, time.Hour, ""); return err },
		"usos demais":    func() error { _, _, err := s.Create(ctx, "x", 101, time.Hour, ""); return err },
		"sem validade":   func() error { _, _, err := s.Create(ctx, "x", 1, 0, ""); return err },
		"validade longa": func() error { _, _, err := s.Create(ctx, "x", 1, 91*24*time.Hour, ""); return err },
		"rótulo longo":   func() error { _, _, err := s.Create(ctx, string(make([]rune, 101)), 1, time.Hour, ""); return err },
	} {
		if call() == nil {
			t.Errorf("%s: deveria ser recusado", name)
		}
	}
}

func TestCodigoNaoFicaGuardadoEmTextoPuro(t *testing.T) {
	s := NewService(testutil.OpenDB(t))
	c, raw := newCode(t, s, 1)
	var hash string
	_ = s.db.QueryRow(`SELECT code_hash FROM signup_codes WHERE id = $1`, c.ID).Scan(&hash)
	if hash == "" || hash == raw {
		t.Errorf("o banco deve guardar só o hash, tem %q", hash)
	}
}
