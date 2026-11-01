package household

import (
	"context"
	"errors"
	"net/http"

	"casal/internal/auth"
	"casal/internal/httpx"
)

type ctxKey struct{}

func IDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok
}

// RequireHousehold deve vir depois de auth.Require. Descobre o casal da pessoa
// logada e o coloca no contexto: é daqui que vem o household_id de todas as consultas.
func RequireHousehold(svc *Service) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := auth.UserFromContext(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "faça login")
				return
			}
			id, err := svc.HouseholdOf(r.Context(), u.ID)
			if errors.Is(err, ErrNoHousehold) {
				httpx.Error(w, http.StatusForbidden, err.Error())
				return
			}
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "erro interno")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
		})
	}
}
