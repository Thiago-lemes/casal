package module

import (
	"log/slog"
	"net/http"

	"casal/internal/httpx"
	"casal/internal/identity"
)

// Require deve vir depois de auth.Require e household.RequireHousehold. Com o módulo desligado,
// responde 403 module_disabled; os dados do módulo continuam guardados.
func Require(svc *Service, log *slog.Logger, name string) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := identity.From(r.Context())
			if !ok || p.HouseholdID == "" {
				httpx.Error(w, http.StatusUnauthorized, "faça login")
				return
			}
			on, err := svc.IsEnabled(r.Context(), p.HouseholdID, name)
			if err != nil {
				httpx.Fail(w, r, log, err)
				return
			}
			if !on {
				httpx.Fail(w, r, log, ErrDisabled)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
