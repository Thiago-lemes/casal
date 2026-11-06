// Package ops reúne a observabilidade do beta: métricas no formato Prometheus, relatório de erros sem conteúdo
// e eventos de produto. Não importa outras features: elas expõem ganchos simples e o app liga tudo.
package ops

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Registry guarda as métricas e as escreve no formato de texto do Prometheus. Escrito à mão (poucas métricas,
// sem dependência nova). Os rótulos são fixos por métrica: nunca receba aqui id de pessoa, de casal nem caminho real.
type Registry struct {
	mu       sync.Mutex
	counters []*CounterVec
	hists    []*HistogramVec
	gauges   []*GaugeVec
	pulls    []func(io.Writer)
}

func NewRegistry() *Registry { return &Registry{} }

// ---- contador ----

type CounterVec struct {
	name, help string
	labels     []string
	mu         sync.Mutex
	vals       map[string]float64
}

func (r *Registry) Counter(name, help string, labels ...string) *CounterVec {
	c := &CounterVec{name: name, help: help, labels: labels, vals: map[string]float64{}}
	r.mu.Lock()
	r.counters = append(r.counters, c)
	r.mu.Unlock()
	return c
}

// Inc soma 1 à série com estes valores de rótulo (na mesma ordem em que foram declarados).
func (c *CounterVec) Inc(values ...string) {
	k := labelKey(c.labels, values)
	c.mu.Lock()
	c.vals[k]++
	c.mu.Unlock()
}

// ---- gauge ----

type GaugeVec struct {
	name, help string
	labels     []string
	mu         sync.Mutex
	vals       map[string]float64
}

func (r *Registry) Gauge(name, help string, labels ...string) *GaugeVec {
	g := &GaugeVec{name: name, help: help, labels: labels, vals: map[string]float64{}}
	r.mu.Lock()
	r.gauges = append(r.gauges, g)
	r.mu.Unlock()
	return g
}

func (g *GaugeVec) Set(v float64, values ...string) {
	k := labelKey(g.labels, values)
	g.mu.Lock()
	g.vals[k] = v
	g.mu.Unlock()
}

// SetNow grava a hora atual em segundos, o formato dos gauges "last_*_timestamp".
func (g *GaugeVec) SetNow(values ...string) { g.Set(float64(time.Now().Unix()), values...) }

// ---- histograma ----

type HistogramVec struct {
	name, help string
	labels     []string
	buckets    []float64
	mu         sync.Mutex
	series     map[string]*histSeries
}

type histSeries struct {
	counts []uint64
	sum    float64
	n      uint64
}

func (r *Registry) Histogram(name, help string, buckets []float64, labels ...string) *HistogramVec {
	h := &HistogramVec{name: name, help: help, labels: labels, buckets: buckets, series: map[string]*histSeries{}}
	r.mu.Lock()
	r.hists = append(r.hists, h)
	r.mu.Unlock()
	return h
}

func (h *HistogramVec) Observe(v float64, values ...string) {
	k := labelKey(h.labels, values)
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.series[k]
	if s == nil {
		s = &histSeries{counts: make([]uint64, len(h.buckets))}
		h.series[k] = s
	}
	for i, b := range h.buckets {
		if v <= b {
			s.counts[i]++
		}
	}
	s.sum += v
	s.n++
}

// OnScrape registra uma função que escreve métricas calculadas na hora da leitura (por exemplo, a fila de lembretes).
func (r *Registry) OnScrape(f func(w io.Writer)) {
	r.mu.Lock()
	r.pulls = append(r.pulls, f)
	r.mu.Unlock()
}

// WriteGauge escreve um gauge simples, sem rótulos; serve para as funções de OnScrape.
func WriteGauge(w io.Writer, name, help string, v float64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %s\n", name, help, name, name, fmtFloat(v))
}

// Handler responde no formato de texto do Prometheus.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.Write(w)
	})
}

func (r *Registry) Write(w io.Writer) {
	r.mu.Lock()
	counters, hists, gauges, pulls := r.counters, r.hists, r.gauges, r.pulls
	r.mu.Unlock()

	for _, c := range counters {
		c.mu.Lock()
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", c.name, c.help, c.name)
		for _, k := range sortedKeys(c.vals) {
			fmt.Fprintf(w, "%s%s %s\n", c.name, k, fmtFloat(c.vals[k]))
		}
		c.mu.Unlock()
	}
	for _, g := range gauges {
		g.mu.Lock()
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n", g.name, g.help, g.name)
		for _, k := range sortedKeys(g.vals) {
			fmt.Fprintf(w, "%s%s %s\n", g.name, k, fmtFloat(g.vals[k]))
		}
		g.mu.Unlock()
	}
	for _, h := range hists {
		h.mu.Lock()
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s histogram\n", h.name, h.help, h.name)
		keys := make([]string, 0, len(h.series))
		for k := range h.series {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s := h.series[k]
			for i, b := range h.buckets {
				fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, withLE(k, fmtFloat(b)), s.counts[i])
			}
			fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, withLE(k, "+Inf"), s.n)
			fmt.Fprintf(w, "%s_sum%s %s\n%s_count%s %d\n", h.name, k, fmtFloat(s.sum), h.name, k, s.n)
		}
		h.mu.Unlock()
	}
	for _, f := range pulls {
		f(w)
	}
}

// labelKey monta o trecho {a="x",b="y"}. Valores a mais ou a menos que os rótulos declarados são um erro de
// programação; em vez de derrubar o servidor, sobram ignorados e faltam como vazio.
func labelKey(names, values []string) string {
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		v := ""
		if i < len(values) {
			v = values[i]
		}
		fmt.Fprintf(&b, "%s=%q", n, escapeLabel(v))
	}
	b.WriteByte('}')
	return b.String()
}

func withLE(key, le string) string {
	if key == "" {
		return `{le="` + le + `"}`
	}
	return key[:len(key)-1] + `,le="` + le + `"}`
}

// escapeLabel só garante que o valor não quebre a linha; %q já escapa aspas e barras.
func escapeLabel(v string) string {
	return strings.ReplaceAll(v, "\n", " ")
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fmtFloat(v float64) string {
	if math.IsInf(v, 1) {
		return "+Inf"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}
