package httpx

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Limiter é um balde de fichas por chave (IP, e-mail...), guardado em memória.
// Cada chave começa com burst fichas e ganha uma nova a cada interval.
// Com várias instâncias do servidor, o limite vale por instância.
type Limiter struct {
	burst    float64
	interval time.Duration
	now      func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

const maxBuckets = 10_000 // acima disso, os baldes ociosos são descartados

func NewLimiter(burst int, interval time.Duration) *Limiter {
	return &Limiter{burst: float64(burst), interval: interval, now: time.Now, buckets: map[string]*bucket{}}
}

// Allow gasta uma ficha da chave. Se não houver, devolve false e quanto esperar.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if len(l.buckets) >= maxBuckets {
		l.purge(now)
	}
	b, found := l.buckets[key]
	if !found {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+float64(now.Sub(b.last))/float64(l.interval))
	b.last = now

	if b.tokens < 1 {
		return false, time.Duration((1 - b.tokens) * float64(l.interval))
	}
	b.tokens--
	return true, 0
}

// purge descarta quem já recuperou todas as fichas (não há nada a lembrar sobre eles).
func (l *Limiter) purge(now time.Time) {
	full := time.Duration(l.burst * float64(l.interval))
	for k, b := range l.buckets {
		if now.Sub(b.last) >= full {
			delete(l.buckets, k)
		}
	}
}

// TooManyRequests responde 429 dizendo em quantos segundos tentar de novo.
func TooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	secs := int(retryAfter.Seconds()) + 1
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	Error(w, http.StatusTooManyRequests, "muitas tentativas, aguarde um pouco")
}

// ClientIP devolve o IP de quem chamou. Atrás de um proxy confiável (trustProxy),
// usa o último valor de X-Forwarded-For, que é o que o próprio proxy acrescentou.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
