package household

import (
	"log/slog"
	"net/http"

	"casal/internal/httpx"
	"casal/internal/identity"
)

// RequireHousehold deve vir depois de auth.Require. Descobre o casal da pessoa
// logada e o coloca no contexto (identity.Principal): é daqui que vem o household_id
// de todas as consultas.
func RequireHousehold(svc *Service, log *slog.Logger) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := identity.From(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "faça login")
				return
			}
			id, err := svc.HouseholdOf(r.Context(), p.UserID)
			if err != nil {
				httpx.Fail(w, r, log, err) // ErrNoHousehold vira 403; o resto, 500 com log
				return
			}
			p.HouseholdID = id
			next.ServeHTTP(w, r.WithContext(identity.With(r.Context(), p)))
		})
	}
}
