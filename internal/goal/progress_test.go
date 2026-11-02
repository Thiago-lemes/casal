package goal

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestWeekdayBit(t *testing.T) {
	// 2026-11-02 é segunda-feira
	for i, want := range []int{1, 2, 4, 8, 16, 32, 64} {
		d := day("2026-11-02").AddDate(0, 0, i)
		if got := weekdayBit(d); got != want {
			t.Errorf("%s (%s): bit %d, quero %d", fmtDay(d), d.Weekday(), got, want)
		}
	}
	if !dayCounts(0, day("2026-11-07")) {
		t.Error("máscara vazia aceita qualquer dia")
	}
	if dayCounts(1|4, day("2026-11-03")) || !dayCounts(1|4, day("2026-11-04")) {
		t.Error("segunda e quarta apenas")
	}
}

func TestPeriodOfWeekRunsMondayToSundayAndCrossesMonths(t *testing.T) {
	// domingo 1/11: a semana é 26/10 (segunda) a 1/11
	s, e := periodOf(PeriodWeek, day("2026-11-01"))
	if fmtDay(s) != "2026-10-26" || fmtDay(e) != "2026-11-02" {
		t.Errorf("semana de domingo: %s a %s", fmtDay(s), fmtDay(e))
	}
	// segunda 2/11 abre a semana seguinte
	s, e = periodOf(PeriodWeek, day("2026-11-02"))
	if fmtDay(s) != "2026-11-02" || fmtDay(e) != "2026-11-09" {
		t.Errorf("semana de segunda: %s a %s", fmtDay(s), fmtDay(e))
	}
	// mês: virada de ano
	s, e = periodOf(PeriodMonth, day("2026-12-31"))
	if fmtDay(s) != "2026-12-01" || fmtDay(e) != "2027-01-01" {
		t.Errorf("mês: %s a %s", fmtDay(s), fmtDay(e))
	}
	if r := rangeOf(s, e); r.End != "2026-12-31" {
		t.Errorf("a ponta final aparece incluída: %+v", r)
	}
}

func TestHabitCountsOnlyAllowedDaysAndTheOwner(t *testing.T) {
	marks := []Checkin{
		{"ana", "2026-11-02"}, {"ana", "2026-11-02"}, // repetida
		{"ana", "2026-11-03"}, // terça: fora da máscara
		{"ana", "2026-11-04"},
		{"bia", "2026-11-04"}, // outra pessoa
		{"ana", "2026-10-31"}, // semana anterior
	}
	start, end := periodOf(PeriodWeek, day("2026-11-04"))
	if n := habitDays(marks, "ana", 0, start, end); n != 3 {
		t.Errorf("qualquer dia: %d marcações, quero 3", n)
	}
	// segunda e quarta: a terça fica no histórico e não conta
	if n := habitDays(marks, "ana", 1|4, start, end); n != 2 {
		t.Errorf("segunda e quarta: %d marcações, quero 2", n)
	}
}

func TestSharedDaysRequiresBothOnlyCountsWhenBothMarked(t *testing.T) {
	members := map[string]bool{"ana": true, "bia": true}
	marks := []Checkin{
		{"ana", "2026-11-03"}, {"bia", "2026-11-03"}, // os dois
		{"ana", "2026-11-05"},                       // só uma
		{"zé", "2026-11-06"}, {"ana", "2026-11-06"}, // zé saiu do casal
	}
	start, end := periodOf(PeriodMonth, day("2026-11-10"))
	n, pending := sharedDays(marks, 1, members, start, end)
	if n != 3 || pending != nil {
		t.Errorf("basta um: %d %v", n, pending)
	}
	n, pending = sharedDays(marks, 2, members, start, end)
	if n != 1 || len(pending) != 2 || pending[0] != "2026-11-05" || pending[1] != "2026-11-06" {
		t.Errorf("os dois: %d pendentes=%v", n, pending)
	}
}

func TestPercentRoundsDown(t *testing.T) {
	for _, c := range []struct {
		done, total int64
		want        int
	}{{0, 10, 0}, {1, 3, 33}, {2, 3, 66}, {3, 3, 100}, {4, 3, 133}, {5, 0, 0}, {-5, 10, 0}} {
		if got := percent(c.done, c.total); got != c.want {
			t.Errorf("percent(%d,%d) = %d, quero %d", c.done, c.total, got, c.want)
		}
	}
}

func TestWholeMonthsBetween(t *testing.T) {
	for _, c := range []struct {
		from, to string
		want     int
	}{
		{"2026-11-02", "2026-12-02", 1},
		{"2026-11-02", "2026-12-01", 0},
		{"2026-11-15", "2027-02-14", 2},
		{"2026-11-15", "2027-02-15", 3},
		{"2026-11-15", "2026-11-10", 0},
	} {
		if got := wholeMonthsBetween(day(c.from), day(c.to)); got != c.want {
			t.Errorf("%s a %s: %d meses, quero %d", c.from, c.to, got, c.want)
		}
	}
}

func TestPaceRoundsUpAndHasAOneMonthFloor(t *testing.T) {
	start, now := day("2026-11-01"), day("2026-11-01")
	// R$ 1.000,00 em 3 meses: 33.334 centavos por mês (arredonda para cima)
	req, _ := pace(100000, 0, start, now, day("2027-02-01"))
	if req != 33334 {
		t.Errorf("por mês: %d, quero 33334", req)
	}
	// falta menos de um mês: conta como um mês
	req, _ = pace(100000, 40000, start, day("2026-11-20"), day("2026-11-25"))
	if req != 60000 {
		t.Errorf("mínimo de um mês: %d, quero 60000", req)
	}
	// meta batida: nada a guardar e no ritmo
	req, ok := pace(100000, 100000, start, now, day("2027-02-01"))
	if req != 0 || !ok {
		t.Errorf("meta batida: %d %v", req, ok)
	}
}

func TestPaceOnTrackComparesWithElapsedFractionOfTheDeadline(t *testing.T) {
	start, due := day("2026-11-01"), day("2026-12-01") // 30 dias
	now := day("2026-11-16")                           // metade do prazo
	if _, ok := pace(100000, 50000, start, now, due); !ok {
		t.Error("50% guardado com 50% do prazo: no ritmo")
	}
	if _, ok := pace(100000, 49000, start, now, due); ok {
		t.Error("49% guardado com 50% do prazo: atrasado")
	}
	// prazo vencido sem bater a meta
	if _, ok := pace(100000, 90000, start, day("2026-12-05"), due); ok {
		t.Error("depois do prazo e sem bater a meta não está no ritmo")
	}
}

func TestBudgetSavingsSumsLeftoverAndMarksCurrentMonthProvisional(t *testing.T) {
	spent := map[string]int64{"2026-09": 30000, "2026-10": 80000, "2026-11": 20000}
	// limite 50.000: set sobra 20.000, out estourou (0, nunca negativo), nov sobra 30.000 até agora
	saved, prov := budgetSavings(50000, spent, day("2026-09-10"), day("2026-11-15"))
	if saved != 50000 || !prov {
		t.Errorf("sobra: %d provisório=%v, quero 50000 true", saved, prov)
	}
	// baseline depois do mês corrente não existe: nenhum mês, nenhuma sobra
	saved, prov = budgetSavings(50000, spent, day("2026-12-01"), day("2026-11-15"))
	if saved != 0 || prov {
		t.Errorf("sem meses: %d %v", saved, prov)
	}
}
