// Package identity carrega, no contexto da requisição, quem está chamando.
// As features leem daqui em vez de importar umas às outras.
package identity

import "context"

// Principal é a pessoa autenticada e o casal dela. HouseholdID fica vazio
// até o middleware de casal rodar.
type Principal struct {
	UserID      string
	HouseholdID string
}

type ctxKey struct{}

func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
