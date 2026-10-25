package auth

import (
	"context"
	"net/http"

	"casal/internal/httpx"
)

const CookieName = "casal_session"

type ctxKey struct{}

func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// Require rejeita com 401 quem não tem sessão válida e coloca o usuário no contexto.
func Require(svc *Service) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(CookieName)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "faça login")
				return
			}
			u, err := svc.UserFromToken(r.Context(), c.Value)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "sessão inválida ou expirada")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
		})
	}
}
