package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelWorksWithErrorsIsAndAs(t *testing.T) {
	notFound := New(NotFound, "não existe")
	wrapped := fmt.Errorf("%w: lembrete 7", notFound)

	if !errors.Is(wrapped, notFound) {
		t.Error("errors.Is deveria reconhecer o sentinela embrulhado")
	}
	var ae *Error
	if !errors.As(wrapped, &ae) || ae.Kind != NotFound {
		t.Errorf("errors.As deveria achar o Kind, veio %+v", ae)
	}
	if wrapped.Error() != "não existe: lembrete 7" {
		t.Errorf("mensagem inesperada: %q", wrapped)
	}
	if errors.Is(wrapped, New(NotFound, "não existe")) {
		t.Error("dois sentinelas diferentes não devem ser iguais, mesmo com a mesma mensagem")
	}
}
