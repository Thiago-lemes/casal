package ops

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"casal/internal/httpx"
)

// HTTPMetrics mede cada requisição pelo padrão da rota (GET /v1/reminders/{id}), nunca pelo caminho real:
// o caminho tem ids e faria a quantidade de séries crescer sem limite.
type HTTPMetrics struct {
	requests *CounterVec
	duration *HistogramVec
}

func NewHTTPMetrics(r *Registry) *HTTPMetrics {
	return &HTTPMetrics{
		requests: r.Counter("http_requests_total", "Requisições atendidas.", "route", "status"),
		duration: r.Histogram("http_request_duration_seconds", "Duração das requisições.",
			[]float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}, "route"),
	}
}

// Wrap mede as requisições que passam pelo mux. O padrão vem do próprio mux (Handler), antes de atender.
func (m *HTTPMetrics) Wrap(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := mux.Handler(r)
		if pattern == "" {
			pattern = "nao_encontrada" // evita uma série por caminho inventado
		}
		httpx.SetRoute(r.Context(), pattern)
		rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		mux.ServeHTTP(rec, r)
		m.requests.Inc(pattern, strconv.Itoa(rec.status))
		m.duration.Observe(time.Since(start).Seconds(), pattern)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// RequireBearer protege /metrics com um token fixo (METRICS_TOKEN), comparado em tempo constante.
func RequireBearer(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(strings.TrimSpace(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "não autorizado", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
