package finance

import (
	"fmt"
	"sort"
	"time"
)

const (
	// MaxAmountCents é o teto de um valor: R$ 10.000.000,00.
	MaxAmountCents = 1_000_000_000
	// PointsTotal é a soma de uma divisão em pontos-base (1% = 100 pontos; 62,5% = 6250).
	PointsTotal = 10000

	minInstallments = 2
	maxInstallments = 48
	dateLayout      = "2006-01-02"
	monthLayout     = "2006-01"
)

// Distribute reparte total em partes proporcionais aos pesos, só com inteiros: cada um fica com
// floor(total × peso ÷ soma) e os que sobram vão um a um para quem perdeu a maior parte fracionária
// (no empate, o menor id). A soma das partes é sempre exatamente total. Pesos zero ficam com zero.
func Distribute(total int64, weights map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(weights))
	var sum int64
	for _, w := range weights {
		sum += w
	}
	if sum <= 0 {
		return out
	}
	type rest struct {
		id string
		r  int64
	}
	rests := make([]rest, 0, len(weights))
	var used int64
	for id, w := range weights {
		q := total * w / sum
		out[id] = q
		used += q
		rests = append(rests, rest{id, total * w % sum})
	}
	sort.Slice(rests, func(i, j int) bool {
		if rests[i].r != rests[j].r {
			return rests[i].r > rests[j].r
		}
		return rests[i].id < rests[j].id
	})
	// o que sobra é menor que o número de pessoas: cada resto fracionário vale menos de 1 centavo
	for i := int64(0); i < total-used; i++ {
		out[rests[i].id]++
	}
	return out
}

// Shares é a parte de cada pessoa num valor, dada a divisão em pontos-base.
func Shares(amount int64, split map[string]int) map[string]int64 {
	w := make(map[string]int64, len(split))
	for id, p := range split {
		w[id] = int64(p)
	}
	return Distribute(amount, w)
}

// EqualSplit divide igualmente entre as pessoas (5000 para cada uma, com duas).
func EqualSplit(members []string) map[string]int {
	w := make(map[string]int64, len(members))
	for _, m := range members {
		w[m] = 1
	}
	return toPoints(Distribute(PointsTotal, w))
}

// ProportionalSplit calcula a divisão pela renda de cada pessoa. Só existe com duas pessoas que informaram
// uma renda maior que zero.
func ProportionalSplit(members []string, incomes map[string]int64) (map[string]int, bool) {
	if len(members) != 2 {
		return nil, false
	}
	w := make(map[string]int64, 2)
	for _, m := range members {
		if incomes[m] <= 0 {
			return nil, false
		}
		w[m] = incomes[m]
	}
	return toPoints(Distribute(PointsTotal, w)), true
}

func toPoints(m map[string]int64) map[string]int {
	out := make(map[string]int, len(m))
	for id, v := range m {
		out[id] = int(v)
	}
	return out
}

// NormalizeSplit confere a divisão (só gente do casal, cada parte de 0 a 10000, soma 10000) e a devolve com
// todas as pessoas do casal, as que faltam com zero.
func NormalizeSplit(split map[string]int, members []string) (map[string]int, error) {
	if len(split) == 0 {
		return nil, fmt.Errorf("%w: informe a divisão", ErrSplitInvalid)
	}
	isMember := make(map[string]bool, len(members))
	for _, m := range members {
		isMember[m] = true
	}
	out := make(map[string]int, len(members))
	for _, m := range members {
		out[m] = 0
	}
	sum := 0
	for id, p := range split {
		if !isMember[id] {
			return nil, fmt.Errorf("%w: a divisão tem alguém que não é do casal", ErrSplitInvalid)
		}
		if p < 0 || p > PointsTotal {
			return nil, fmt.Errorf("%w: cada parte vai de 0 a %d", ErrSplitInvalid, PointsTotal)
		}
		out[id] = p
		sum += p
	}
	if sum != PointsTotal {
		return nil, fmt.Errorf("%w: a divisão precisa somar %d", ErrSplitInvalid, PointsTotal)
	}
	return out, nil
}

// SplitInstallments reparte o total em count parcelas: cada uma vale floor(total ÷ count) e os centavos que
// sobram vão um a um para as primeiras. A soma é exatamente o total.
func SplitInstallments(total int64, count int) []int64 {
	out := make([]int64, count)
	base, extra := total/int64(count), total%int64(count)
	for i := range out {
		out[i] = base
		if int64(i) < extra {
			out[i]++
		}
	}
	return out
}

// AddMonths soma meses a uma data mantendo o dia âncora; em mês mais curto vale o último dia
// (dia 31 vira 28, 29 ou 30). A âncora é o dia da data original, não o do mês anterior: 31/jan -> 28/fev -> 31/mar.
func AddMonths(t time.Time, months, anchorDay int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	if anchorDay > last {
		anchorDay = last
	}
	return time.Date(first.Year(), first.Month(), anchorDay, 0, 0, 0, 0, time.UTC)
}

// DayInMonth é o dia day no mês de ref, ou o último dia dele se o mês for mais curto.
func DayInMonth(ref time.Time, day int) time.Time { return AddMonths(ref, 0, day) }

func parseDate(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil || t.Year() < 2000 || t.Year() > 2100 {
		return time.Time{}, fmt.Errorf("%w: data inválida (use AAAA-MM-DD)", ErrInvalidInput)
	}
	return t, nil
}

// parseMonth lê AAAA-MM e devolve o primeiro dia do mês.
func parseMonth(s string) (time.Time, error) {
	t, err := time.Parse(monthLayout, s)
	if err != nil || t.Year() < 2000 || t.Year() > 2100 {
		return time.Time{}, fmt.Errorf("%w: mês inválido (use AAAA-MM)", ErrInvalidInput)
	}
	return t, nil
}

// today é a data de hoje no fuso da pessoa (fuso inválido ou vazio cai em São Paulo).
func today(now time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" || tz == "Local" {
		loc, _ = time.LoadLocation("America/Sao_Paulo")
		if loc == nil {
			loc = time.UTC
		}
	}
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func fmtDate(t time.Time) string { return t.Format(dateLayout) }

func validAmount(c int64) bool { return c > 0 && c <= MaxAmountCents }
