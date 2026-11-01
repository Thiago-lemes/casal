package token

import "testing"

func TestNewReturnsRawAndMatchingHash(t *testing.T) {
	raw, hash, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if raw == hash || hash != Hash(raw) {
		t.Error("o hash deve ser derivado do token e diferente dele")
	}
	if len(raw) < 40 {
		t.Errorf("token curto demais para 256 bits: %d caracteres", len(raw))
	}
	other, _, _ := New()
	if other == raw {
		t.Error("dois tokens não podem ser iguais")
	}
}

func TestHashIsStable(t *testing.T) {
	first, second, other := Hash("abc"), Hash("abc"), Hash("abd")
	if first != second || first == other {
		t.Error("o hash deve ser determinístico e sensível à entrada")
	}
}
