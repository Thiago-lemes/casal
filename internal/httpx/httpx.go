package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type ErrorBody struct {
	Code  string `json:"code,omitempty"`
	Error string `json:"error"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, ErrorBody{Error: msg})
}

// ErrorCode é Error com um código estável, que o cliente traduz em vez de mostrar a mensagem.
func ErrorCode(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, ErrorBody{Code: code, Error: msg})
}

// Decode lê o corpo JSON com limite de 1 MB e rejeita campos desconhecidos.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

type Middleware func(http.Handler) http.Handler

// Chain aplica os middlewares de fora para dentro: o primeiro da lista é o mais externo.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic", "err", rec, "path", r.URL.Path, "request_id", RequestIDFrom(r.Context()))
					reportServerError(r, nil, rec)
					Error(w, http.StatusInternalServerError, "erro interno")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func Logging(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info("request", "request_id", RequestIDFrom(r.Context()), "method", r.Method, "path", r.URL.Path,
				"status", rec.status, "dur", time.Since(start).String())
		})
	}
}
