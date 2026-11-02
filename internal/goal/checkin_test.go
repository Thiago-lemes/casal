package goal

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"casal/internal/identity"
)

func (e *env) mark(p identity.Principal, goalID, day string) CheckinResult {
	e.t.Helper()
	r, err := e.svc.Checkin(ctx(), p, goalID, day)
	mustNoErr(e.t, err)
	return r
}

func TestCheckinIsIdempotentPerDay(t *testing.T) {
	e := newEnv(t)
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3})
	first := e.mark(e.ana, g.ID, "")
	again := e.mark(e.ana, g.ID, "2026-11-15") // hoje, explícito
	if !first.Created || again.Created {
		t.Errorf("created: %v e %v", first.Created, again.Created)
	}
	if again.Goal.Progress.Done != 1 || !again.Goal.Progress.MarkedToday {
		t.Errorf("duas marcações no mesmo dia contam uma: %+v", again.Goal.Progress)
	}
}

func TestCheckinDateLimits(t *testing.T) {
	e := newEnv(t)
	g := e.habit(e.ana, "Correr", HabitInput{})
	for _, c := range []struct{ day, why string }{
		{"2026-11-16", "futuro"}, {"2026-10-14", "32 dias atrás"}, {"ontem", "malformada"},
	} {
		_, err := e.svc.Checkin(ctx(), e.ana, g.ID, c.day)
		wantErr(t, err, ErrCheckinDate, c.why)
	}
	e.mark(e.ana, g.ID, "2026-10-15") // 31 dias atrás ainda vale
}

func TestCheckinDayFollowsTheTimezoneOfWhoMarks(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Caminhada", SharedInput{TargetCount: 5, Period: PeriodMonth})
	// 02:00 UTC de 16/11: em São Paulo ainda é 15/11 (23h); em Tóquio já é 16/11 (11h)
	e.now = time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC)
	tokyo := e.beto
	tokyo.Timezone = "Asia/Tokyo"
	e.mark(e.ana, g.ID, "")
	e.mark(tokyo, g.ID, "")

	h, err := e.svc.History(ctx(), e.ana, g.ID, "2026-11-01", "2026-11-30")
	mustNoErr(t, err)
	days := map[string][]string{}
	for _, d := range h.Days {
		days[d.Day] = d.By
	}
	if len(days["2026-11-15"]) != 1 || days["2026-11-15"][0] != e.ana.UserID ||
		len(days["2026-11-16"]) != 1 || days["2026-11-16"][0] != e.beto.UserID {
		t.Errorf("o dia gravado é o do fuso de quem marca: %v", days)
	}
}

func TestHabitWeekRunsMondayToSundayAndResetsOnMonday(t *testing.T) {
	e := newEnv(t)
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3, Period: PeriodWeek})
	// hoje é domingo 15/11: a semana é 9/11 a 15/11
	e.mark(e.ana, g.ID, "2026-11-09")
	e.mark(e.ana, g.ID, "2026-11-15")
	e.mark(e.ana, g.ID, "2026-11-08") // domingo da semana anterior: fora
	got, _ := e.svc.Get(ctx(), e.ana, g.ID)
	if got.Progress.Done != 2 || got.Progress.Percent != 66 || got.Progress.Period.Start != "2026-11-09" || got.Progress.Period.End != "2026-11-15" {
		t.Errorf("semana de domingo: %+v", got.Progress)
	}
	// segunda 16/11 abre a semana seguinte
	e.now = time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC)
	got, _ = e.svc.Get(ctx(), e.ana, g.ID)
	if got.Progress.Done != 0 || got.Progress.Period.Start != "2026-11-16" {
		t.Errorf("segunda: %+v", got.Progress)
	}
}

func TestMonthlyPeriodRollsOverAtTheMonthBoundary(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Jantar fora", SharedInput{TargetCount: 2, Period: PeriodMonth})
	e.mark(e.ana, g.ID, "2026-11-10")
	e.mark(e.beto, g.ID, "2026-11-12")
	got, _ := e.svc.Get(ctx(), e.ana, g.ID)
	if got.Progress.Done != 2 || got.Progress.Percent != 100 {
		t.Fatalf("novembro: %+v", got.Progress)
	}
	e.now = time.Date(2026, 12, 1, 15, 0, 0, 0, time.UTC)
	got, _ = e.svc.Get(ctx(), e.ana, g.ID)
	if got.Progress.Done != 0 || got.Progress.Period.Start != "2026-12-01" || got.Progress.Period.End != "2026-12-31" {
		t.Errorf("dezembro: %+v", got.Progress)
	}
	// o histórico guarda novembro, em fatos
	h, err := e.svc.History(ctx(), e.ana, g.ID, "2026-11-01", "2026-12-31")
	mustNoErr(t, err)
	if len(h.Periods) != 2 || h.Periods[0].Count != 2 || h.Periods[0].Target != 2 || h.Periods[1].Count != 0 {
		t.Errorf("períodos: %+v", h.Periods)
	}
}

func TestHabitWeekdaysOutsideTheMaskStayInHistoryButDoNotCount(t *testing.T) {
	e := newEnv(t)
	// segunda (1) e quarta (4)
	g := e.habit(e.ana, "Academia", HabitInput{TargetCount: 2, Period: PeriodWeek, Weekdays: 1 | 4})
	e.mark(e.ana, g.ID, "2026-11-09") // segunda
	e.mark(e.ana, g.ID, "2026-11-10") // terça: fora dos dias
	got, _ := e.svc.Get(ctx(), e.ana, g.ID)
	if got.Progress.Done != 1 {
		t.Errorf("a terça não conta: %+v", got.Progress)
	}
	h, _ := e.svc.History(ctx(), e.ana, g.ID, "2026-11-09", "2026-11-15")
	counted := map[string]bool{}
	for _, d := range h.Days {
		counted[d.Day] = d.Counted
	}
	if len(h.Days) != 2 || !counted["2026-11-09"] || counted["2026-11-10"] {
		t.Errorf("a terça aparece no histórico sem contar: %+v", h.Days)
	}
}

func TestPausingDoesNotZeroAnything(t *testing.T) {
	e := newEnv(t)
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3})
	e.mark(e.ana, g.ID, "")
	p, err := e.svc.Pause(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	if p.Progress.Done != 1 {
		t.Errorf("pausar zerou o progresso: %+v", p.Progress)
	}
	// pausada ainda aceita a marcação de um dia que passou (só concluída e arquivada fecham)
	e.mark(e.ana, g.ID, "2026-11-14")
	r, _ := e.svc.Resume(ctx(), e.ana, g.ID)
	if r.Progress.Done != 2 {
		t.Errorf("depois de retomar: %+v", r.Progress)
	}
}

func TestSharedRequiresBothCountsOnlyWhenBothMark(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Cozinhar juntos", SharedInput{TargetCount: 2, Period: PeriodMonth, RequiresBoth: true})
	r := e.mark(e.ana, g.ID, "2026-11-10")
	if r.Goal.Progress.Done != 0 || len(r.Goal.Progress.PendingDays) != 1 || r.Goal.Progress.PendingDays[0] != "2026-11-10" {
		t.Fatalf("só uma marcou: %+v", r.Goal.Progress)
	}
	r = e.mark(e.beto, g.ID, "2026-11-10")
	if r.Goal.Progress.Done != 1 || len(r.Goal.Progress.PendingDays) != 0 {
		t.Errorf("os dois marcaram: %+v", r.Goal.Progress)
	}
	// com um só marcando, a meta que exige um basta
	one := e.shared(e.ana, "Um basta", SharedInput{TargetCount: 2, Period: PeriodMonth})
	r = e.mark(e.beto, one.ID, "2026-11-10")
	if r.Goal.Progress.Done != 1 {
		t.Errorf("basta um: %+v", r.Goal.Progress)
	}
}

func TestSharedNoticeIsNeutralOncePerDayAndNeverToWhoMarked(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Jantar fora", SharedInput{})

	e.mark(e.ana, g.ID, "")
	got := e.notes.to(e.beto.UserID)
	if len(got) != 1 || got[0].Body != "Ana marcou: Jantar fora" {
		t.Fatalf("aviso ao parceiro: %+v", got)
	}
	if len(e.notes.to(e.ana.UserID)) != 0 {
		t.Error("quem marcou não recebe aviso")
	}
	// o parceiro marca no mesmo dia: nenhum aviso novo (no máximo um por meta por dia)
	e.mark(e.beto, g.ID, "")
	e.mark(e.ana, g.ID, "") // idempotente, nem chega ao aviso
	if e.notes.count() != 1 {
		t.Errorf("avisos no dia: %d, quero 1", e.notes.count())
	}
	// marcar um dia que passou não avisa
	e.mark(e.ana, g.ID, "2026-11-10")
	if e.notes.count() != 1 {
		t.Errorf("marcação de dia passado avisou: %d", e.notes.count())
	}
	// no dia seguinte, avisa de novo
	e.now = e.now.AddDate(0, 0, 1)
	e.mark(e.ana, g.ID, "")
	if len(e.notes.to(e.beto.UserID)) != 2 {
		t.Errorf("no dia seguinte: %+v", e.notes.to(e.beto.UserID))
	}
	// meta pausada e hábito pessoal não avisam ninguém
	_, _ = e.svc.Pause(ctx(), e.ana, g.ID)
	e.now = e.now.AddDate(0, 0, 1)
	e.mark(e.ana, g.ID, "")
	h := e.habit(e.ana, "Ler", HabitInput{})
	e.mark(e.ana, h.ID, "")
	if e.notes.count() != 2 {
		t.Errorf("pausada e pessoal não avisam: %d", e.notes.count())
	}
}

func TestUndoRemovesOnlyYourOwnMark(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Jantar fora", SharedInput{TargetCount: 5})
	e.mark(e.ana, g.ID, "2026-11-10")
	e.mark(e.beto, g.ID, "2026-11-11")

	// o Beto tenta desfazer o dia da Ana: nada acontece
	got, err := e.svc.UndoCheckin(ctx(), e.beto, g.ID, "2026-11-10")
	mustNoErr(t, err)
	if got.Progress.Done != 2 {
		t.Errorf("desfez a marcação do parceiro: %+v", got.Progress)
	}
	got, err = e.svc.UndoCheckin(ctx(), e.beto, g.ID, "2026-11-11")
	mustNoErr(t, err)
	if got.Progress.Done != 1 {
		t.Errorf("a própria marcação some: %+v", got.Progress)
	}
	if _, err = e.svc.UndoCheckin(ctx(), e.beto, g.ID, "2026-11-11"); err != nil {
		t.Errorf("desfazer o que não existe não é erro: %v", err)
	}
}

func TestOnlyTheOwnerChecksAHabit(t *testing.T) {
	e := newEnv(t)
	g, err := e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindHabit, Scope: ScopePersonal, Visibility: VisibilityView, Title: "Meditar",
		Habit: &HabitInput{TargetCount: 3, Period: PeriodWeek}})
	mustNoErr(t, err)
	_, err = e.svc.Checkin(ctx(), e.beto, g.ID, "")
	wantErr(t, err, ErrForbidden, "o parceiro marcando hábito view")
	_, err = e.svc.UndoCheckin(ctx(), e.beto, g.ID, "2026-11-15")
	wantErr(t, err, ErrForbidden, "o parceiro desfazendo hábito view")
	_, err = e.svc.History(ctx(), e.beto, g.ID, "", "")
	wantErr(t, err, ErrForbidden, "o parceiro lendo o histórico dia a dia")

	// ele vê o progresso, sem as marcações do dia
	e.mark(e.ana, g.ID, "")
	seen, _ := e.svc.Get(ctx(), e.beto, g.ID)
	if seen.Progress.Done != 1 || seen.Progress.MarkedToday || seen.Recent != nil || seen.Habit != nil {
		t.Errorf("visão do parceiro: %+v", seen)
	}
}

func TestKindMismatchOnCheckinAndHistory(t *testing.T) {
	e := newEnv(t)
	f := e.saving(e.ana, ScopePersonal, "Reserva", 1000)
	_, err := e.svc.Checkin(ctx(), e.ana, f.ID, "")
	wantErr(t, err, ErrKindMismatch, "marcação em meta financeira")
	_, err = e.svc.History(ctx(), e.ana, f.ID, "", "")
	wantErr(t, err, ErrKindMismatch, "histórico de meta financeira")

	h := e.habit(e.ana, "Correr", HabitInput{})
	_, err = e.svc.History(ctx(), e.ana, h.ID, "2025-01-01", "2026-12-31")
	wantErr(t, err, ErrInvalid, "intervalo de mais de 366 dias")
	_, err = e.svc.History(ctx(), e.ana, h.ID, "2026-11-10", "2026-11-01")
	wantErr(t, err, ErrInvalid, "intervalo invertido")
}

// Nenhuma resposta ordena, soma por pessoa ou compara os dois: o produto promete "sem ranking nem cobrança".
func TestNoResponseRanksOrComparesThePeople(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Jantar fora", SharedInput{TargetCount: 4, RequiresBoth: true})
	e.mark(e.ana, g.ID, "2026-11-03")
	e.mark(e.ana, g.ID, "2026-11-04")
	e.mark(e.beto, g.ID, "2026-11-03")

	detail, _ := e.svc.Get(ctx(), e.ana, g.ID)
	list, _ := e.svc.List(ctx(), e.ana, ListFilter{})
	hist, _ := e.svc.History(ctx(), e.ana, g.ID, "2026-11-01", "2026-11-30")
	ex, _ := e.svc.Export(ctx(), e.ana)
	for name, v := range map[string]any{"detalhe": detail, "lista": list, "histórico": hist, "exportação": ex} {
		raw, err := json.Marshal(v)
		mustNoErr(t, err)
		low := strings.ToLower(string(raw))
		for _, banned := range []string{"rank", "score", "streak", "leader", "winner", "behind", "late_user", "by_user", "per_user", "count_by", "total_by", "points"} {
			if strings.Contains(low, banned) {
				t.Errorf("%s traz %q, que compara ou cobra as pessoas: %s", name, banned, raw)
			}
		}
	}
	// o "quem marcou" de cada dia vem em ordem de id (sem significado) e sem nenhum total por pessoa
	for _, d := range hist.Days {
		for i := 1; i < len(d.By); i++ {
			if d.By[i-1] > d.By[i] {
				t.Errorf("By deveria vir em ordem de id: %v", d.By)
			}
		}
	}
	// e o aviso nunca diz que o outro está atrasado
	for _, m := range e.notes.sent {
		for _, bad := range []string{"atras", "falta", "cobr", "ainda não"} {
			if strings.Contains(strings.ToLower(m.Body), bad) {
				t.Errorf("aviso cobra: %q", m.Body)
			}
		}
	}
}
