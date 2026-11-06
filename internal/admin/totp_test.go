package admin

import (
	"testing"
	"time"
)

// Vetores do RFC 6238 (apêndice B, SHA-1, segredo ASCII "12345678901234567890"), em 6 dígitos.
func TestTOTPVetoresDoRFC(t *testing.T) {
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	for unix, want := range map[int64]string{
		59:         "287082",
		1111111109: "081804",
		1111111111: "050471",
		1234567890: "005924",
		2000000000: "279037",
	} {
		got, err := totpCode(secret, unix/30)
		if err != nil || got != want {
			t.Errorf("t=%d: %q (%v), esperado %q", unix, got, err, want)
		}
	}
}

func TestTOTPJanelaEReaproveitamento(t *testing.T) {
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	cur := now.Unix() / 30
	codeAt := func(step int64) string {
		c, err := totpCode(secret, step)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	// aceita o passo atual e os vizinhos (relógio atrasado ou adiantado)
	for _, s := range []int64{cur - 1, cur, cur + 1} {
		if step, ok := verifyTOTP(secret, codeAt(s), now, 0); !ok || step != s {
			t.Errorf("passo %d: ok=%v step=%d", s, ok, step)
		}
	}
	// fora da janela
	for _, s := range []int64{cur - 2, cur + 2} {
		if _, ok := verifyTOTP(secret, codeAt(s), now, 0); ok {
			t.Errorf("passo %d fora da janela foi aceito", s)
		}
	}
	// o código que já entrou não entra de novo, nem um mais antigo
	if _, ok := verifyTOTP(secret, codeAt(cur), now, cur); ok {
		t.Error("o mesmo passo foi aceito duas vezes")
	}
	if _, ok := verifyTOTP(secret, codeAt(cur-1), now, cur); ok {
		t.Error("um passo mais antigo que o último aceito foi aceito")
	}
	// formato
	for _, bad := range []string{"", "12345", "1234567", "abcdef", " "} {
		if _, ok := verifyTOTP(secret, bad, now, 0); ok {
			t.Errorf("%q não deveria passar", bad)
		}
	}
}
