package goal

import (
	"sort"
	"time"
)

// Todas as datas daqui são "dias": time.Time à meia-noite UTC, sem fuso. O fuso entra só em today().

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

func location(tz string) *time.Location {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" || tz == "Local" {
		loc, _ = time.LoadLocation("America/Sao_Paulo")
		if loc == nil {
			loc = time.UTC
		}
	}
	return loc
}

func fmtDay(t time.Time) string { return t.Format(dateLayout) }

func parseDay(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, ErrInvalid
	}
	return t, nil
}

// weekdayBit é o bit do dia na máscara: segunda = 1, ... domingo = 64.
func weekdayBit(day time.Time) int {
	return 1 << ((int(day.Weekday()) + 6) % 7)
}

// dayCounts diz se um dia vale para a meta: máscara vazia aceita qualquer dia.
func dayCounts(mask int, day time.Time) bool { return mask == 0 || mask&weekdayBit(day) != 0 }

// periodOf devolve o período que contém o dia: a semana civil de segunda a domingo, ou o mês civil.
// O fim não está incluído.
func periodOf(period string, day time.Time) (start, end time.Time) {
	if period == PeriodMonth {
		start = time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0)
	}
	offset := (int(day.Weekday()) + 6) % 7
	start = time.Date(day.Year(), day.Month(), day.Day()-offset, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 0, 7)
}

func rangeOf(start, end time.Time) *Range {
	return &Range{Start: fmtDay(start), End: fmtDay(end.AddDate(0, 0, -1))}
}

func percent(done, total int64) int {
	if total <= 0 || done <= 0 {
		return 0
	}
	return int(done * 100 / total)
}

func inPeriod(day, start, end time.Time) bool { return !day.Before(start) && day.Before(end) }

// habitDays conta os dias distintos marcados pelo dono dentro do período. Só contam os dias permitidos pela
// máscara: uma marcação fora deles fica no histórico e não entra no progresso.
func habitDays(marks []Checkin, owner string, mask int, start, end time.Time) (counted int) {
	seen := map[string]bool{}
	for _, m := range marks {
		if m.UserID != owner || seen[m.Day] {
			continue
		}
		d, err := parseDay(m.Day)
		if err != nil || !inPeriod(d, start, end) || !dayCounts(mask, d) {
			continue
		}
		seen[m.Day] = true
	}
	return len(seen)
}

// sharedDays conta as ocorrências da meta do casal no período. required é quantas pessoas precisam marcar o dia
// (1: basta uma; 2: as duas). Devolve também os dias em que só uma marcou quando as duas são exigidas.
func sharedDays(marks []Checkin, required int, members map[string]bool, start, end time.Time) (count int, pending []string) {
	byDay := map[string]map[string]bool{}
	for _, m := range marks {
		// só contam as pessoas que ainda são do casal: quem saiu não trava a meta (veja HandleLeave)
		if !members[m.UserID] {
			continue
		}
		d, err := parseDay(m.Day)
		if err != nil || !inPeriod(d, start, end) {
			continue
		}
		if byDay[m.Day] == nil {
			byDay[m.Day] = map[string]bool{}
		}
		byDay[m.Day][m.UserID] = true
	}
	for day, who := range byDay {
		if len(who) >= required {
			count++
		} else if required > 1 {
			pending = append(pending, day)
		}
	}
	sort.Strings(pending)
	return count, pending
}

// wholeMonthsBetween são os meses inteiros de from até to (to antes de from dá 0).
func wholeMonthsBetween(from, to time.Time) int {
	if !to.After(from) {
		return 0
	}
	months := (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
	if to.Day() < from.Day() {
		months--
	}
	if months < 0 {
		return 0
	}
	return months
}

// pace calcula o ritmo de uma meta financeira com prazo: quanto guardar por mês e se o guardado já passa da fração
// do prazo decorrido. Sem prazo, não há ritmo.
func pace(target, saved int64, start, now, due time.Time) (required int64, onTrack bool) {
	months := int64(wholeMonthsBetween(now, due))
	if months < 1 {
		months = 1 // mínimo de um mês, inclusive depois do prazo
	}
	if missing := target - saved; missing > 0 {
		required = (missing + months - 1) / months // arredonda para cima
	}
	total := due.Sub(start).Hours() / 24
	elapsed := now.Sub(start).Hours() / 24
	switch {
	case saved >= target:
		onTrack = true
	case total <= 0 || !now.Before(due):
		onTrack = false
	default:
		if elapsed < 0 {
			elapsed = 0
		}
		// guardado / alvo >= decorrido / prazo, sem ponto flutuante
		onTrack = float64(saved)*total >= float64(target)*elapsed
	}
	return required, onTrack
}

// monthsSpan lista os meses (AAAA-MM) de from até to, inclusive.
func monthsSpan(from, to time.Time) []string {
	var out []string
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cur.After(last) && len(out) < 600 {
		out = append(out, cur.Format("2006-01"))
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

// budgetSavings soma, mês a mês, o que sobrou do limite: max(0, limite − gasto). O mês corrente é provisório.
func budgetSavings(limit int64, spent map[string]int64, from, now time.Time) (saved int64, provisional bool) {
	current := now.Format("2006-01")
	for _, m := range monthsSpan(from, now) {
		if left := limit - spent[m]; left > 0 {
			saved += left
		}
		if m == current {
			provisional = true
		}
	}
	return saved, provisional
}

// monthStart é o primeiro dia do mês do dia informado.
func monthStart(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
}
