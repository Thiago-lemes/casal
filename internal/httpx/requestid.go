package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type requestIDKey struct{}

// RequestID dá a cada requisição um identificador, devolvido no cabeçalho X-Request-Id
// e presente nos logs, para ligar a queixa de uma pessoa à linha do log.
// O valor enviado pelo cliente é ignorado de propósito (evita injeção em logs).
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id := hex.EncodeToString(b)
			w.Header().Set("X-Request-Id", id)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
		})
	}
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
