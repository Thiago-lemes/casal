package routine

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func mask(t *testing.T, days ...int) uint8 {
	t.Helper()
	m, err := MaskFromDays(days)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func dates(os []Occurrence, loc *time.Location) []string {
	out := []string{}
	for _, o := range os {
		out = append(out, o.Start.In(loc).Format("2006-01-02 15:04"))
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var sp = location("America/Sao_Paulo")

// 2026-11-02 é uma segunda-feira.
func weekly(t *testing.T, interval int, days ...int) Block {
	return Block{
		ID: "b1", OwnerID: "ana", OwnerName: "Ana", OwnerTZ: "America/Sao_Paulo", Title: "Trabalho",
		Recurrence: RecurrenceWeekly, Weekdays: mask(t, days...), IntervalWeeks: interval,
		StartDate: day("2026-11-02"), StartMin: 9 * 60, EndMin: 18 * 60,
	}
}

func TestWeeklyOccurrences(t *testing.T) {
	b := weekly(t, 1, 1, 3, 5) // segunda, quarta e sexta
	got := dates(Occurrences(b, nil, day("2026-11-02"), day("2026-11-13")), sp)
	want := []string{
		"2026-11-02 09:00", "2026-11-04 09:00", "2026-11-06 09:00",
		"2026-11-09 09:00", "2026-11-11 09:00", "2026-11-13 09:00",
	}
	if !equal(got, want) {
		t.Errorf("veio %v, queria %v", got, want)
	}
}

func TestOccurrencesRespectStartAndEndDate(t *testing.T) {
	b := weekly(t, 1, 1)
	end := day("2026-11-16")
	b.EndDate = &end
	got := dates(Occurrences(b, nil, day("2026-10-26"), day("2026-11-30")), sp)
	want := []string{"2026-11-02 09:00", "2026-11-09 09:00", "2026-11-16 09:00"}
	if !equal(got, want) {
		t.Errorf("antes do início e depois do fim não vale: veio %v, queria %v", got, want)
	}
}

func TestAlternateWeeks(t *testing.T) {
	b := weekly(t, 2, 1, 4) // segunda e quinta, semana sim, semana não
	got := dates(Occurrences(b, nil, day("2026-11-02"), day("2026-11-22")), sp)
	want := []string{"2026-11-02 09:00", "2026-11-05 09:00", "2026-11-16 09:00", "2026-11-19 09:00"}
	if !equal(got, want) {
		t.Errorf("veio %v, queria %v", got, want)
	}
}

func TestAlternateWeeksAnchorFollowsStartDate(t *testing.T) {
	// ancorado numa quarta: a semana "sim" é a dessa quarta, mesmo que a segunda dela fique antes do início
	b := weekly(t, 2, 1, 3)
	b.StartDate = day("2026-11-04")
	got := dates(Occurrences(b, nil, day("2026-11-01"), day("2026-11-22")), sp)
	want := []string{"2026-11-04 09:00", "2026-11-16 09:00", "2026-11-18 09:00"}
	if !equal(got, want) {
		t.Errorf("veio %v, queria %v", got, want)
	}
}

func TestOnceOnlyHappensOnItsDate(t *testing.T) {
	b := weekly(t, 1)
	b.Recurrence, b.Weekdays, b.StartDate = RecurrenceOnce, 0, day("2026-11-05")
	got := dates(Occurrences(b, nil, day("2026-11-01"), day("2026-11-30")), sp)
	if !equal(got, []string{"2026-11-05 09:00"}) {
		t.Errorf("veio %v", got)
	}
}

func TestExceptions(t *testing.T) {
	b := weekly(t, 1, 1, 3)
	ex := map[string]Exception{
		"2026-11-02": {BlockID: "b1", Date: day("2026-11-02"), Skipped: true},
		"2026-11-04": {BlockID: "b1", Date: day("2026-11-04"), StartMin: 13 * 60, EndMin: 17 * 60},
	}
	occ := Occurrences(b, ex, day("2026-11-02"), day("2026-11-04"))
	if len(occ) != 1 {
		t.Fatalf("a folga deveria tirar o dia 2: %v", dates(occ, sp))
	}
	if occ[0].Start.In(sp).Hour() != 13 || occ[0].End.In(sp).Hour() != 17 || !occ[0].Exception {
		t.Errorf("a exceção deveria trocar o horário: %+v", occ[0])
	}
	// sem a exceção, a rotina fixa continua igual
	if len(Occurrences(b, nil, day("2026-11-02"), day("2026-11-04"))) != 2 {
		t.Error("a rotina fixa não pode mudar")
	}
}

func TestOccurrencesUseOwnerTimezone(t *testing.T) {
	b := weekly(t, 1, 1)
	b.OwnerTZ = "America/Manaus" // UTC-4; São Paulo é UTC-3
	o := Occurrences(b, nil, day("2026-11-02"), day("2026-11-02"))[0]
	if o.Start.UTC().Hour() != 13 {
		t.Errorf("09:00 em Manaus são 13:00 UTC, veio %v", o.Start.UTC())
	}
	b.OwnerTZ = "America/Sao_Paulo"
	o = Occurrences(b, nil, day("2026-11-02"), day("2026-11-02"))[0]
	if o.Start.UTC().Hour() != 12 {
		t.Errorf("09:00 em São Paulo são 12:00 UTC, veio %v", o.Start.UTC())
	}
}

func TestEndAtMidnight(t *testing.T) {
	b := weekly(t, 1, 1)
	b.StartMin, b.EndMin = 22*60, 1440
	o := Occurrences(b, nil, day("2026-11-02"), day("2026-11-02"))[0]
	if o.End.In(sp).Format("2006-01-02 15:04") != "2026-11-03 00:00" {
		t.Errorf("24:00 deveria ser a meia-noite seguinte: %v", o.End.In(sp))
	}
}

func TestProject(t *testing.T) {
	b := Block{OwnerID: "ana", Title: "Terapia", Category: "appointment", Location: "clínica"}
	cases := []struct {
		name       string
		visibility string
		viewer     string
		visible    bool
		busy       bool
		title      string
	}{
		{"dono vê tudo, mesmo privado", VisibilityPrivate, "ana", true, false, "Terapia"},
		{"parceiro não vê o privado", VisibilityPrivate, "beto", false, false, ""},
		{"parceiro vê só ocupado", VisibilityBusy, "beto", true, true, ""},
		{"parceiro vê os detalhes", VisibilityView, "beto", true, false, "Terapia"},
		{"parceiro vê o bloco do casal", VisibilityHousehold, "beto", true, false, "Terapia"},
	}
	for _, c := range cases {
		b.Visibility = c.visibility
		got, ok := Project(b, c.viewer)
		if ok != c.visible || got.Busy != c.busy || got.Title != c.title {
			t.Errorf("%s: veio ok=%v busy=%v título=%q", c.name, ok, got.Busy, got.Title)
		}
		if c.busy && (got.Category != "" || got.Location != "") {
			t.Errorf("%s: vazou categoria ou local: %+v", c.name, got)
		}
	}
}

func TestProjectKeepsScheduleSoOccurrencesStayRight(t *testing.T) {
	b := weekly(t, 1, 1)
	b.OwnerID, b.Visibility = "ana", VisibilityBusy
	p, _ := Project(b, "beto")
	occ := Occurrences(p, nil, day("2026-11-02"), day("2026-11-02"))
	if len(occ) != 1 || !occ[0].Busy || occ[0].Title != "" {
		t.Errorf("a ocorrência do parceiro deveria sair como ocupado: %+v", occ)
	}
}

func TestOverlaps(t *testing.T) {
	a := weekly(t, 1, 1, 3)
	b := weekly(t, 1, 3)
	b.ID, b.StartMin, b.EndMin = "b2", 17*60, 19*60 // quarta, 17h às 19h: invade o fim do trabalho
	from, to := day("2026-11-02"), day("2026-12-27")
	if !Overlaps(a, b, from, to) {
		t.Error("deveria se sobrepor na quarta")
	}
	b.StartMin = 18 * 60 // começa quando o outro termina: encostar não é sobrepor
	if Overlaps(a, b, from, to) {
		t.Error("encostar não é sobrepor")
	}
	c := weekly(t, 1, 2) // terça: outro dia
	c.ID = "b3"
	if Overlaps(a, c, from, to) {
		t.Error("dias diferentes não se sobrepõem")
	}
	d := weekly(t, 2, 1) // segunda, semana sim semana não
	d.ID, d.StartDate = "b4", day("2026-11-09")
	if Overlaps(weekly(t, 2, 1), d, from, to) {
		t.Error("semanas alternadas opostas não se sobrepõem")
	}
}

func TestFreeWindows(t *testing.T) {
	base := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	h := func(n int) time.Time { return base.Add(time.Duration(n) * time.Hour) }
	busy := []Interval{
		{h(13), h(15)},
		{h(9), h(12)},  // fora de ordem
		{h(10), h(11)}, // dentro de outro
		{h(14), h(16)}, // emenda com o anterior
		{h(5), h(8)},   // começa antes da janela
	}
	got := FreeWindows(busy, h(7), h(22), 30*time.Minute)
	want := []Interval{{h(8), h(9)}, {h(12), h(13)}, {h(16), h(22)}}
	if len(got) != len(want) {
		t.Fatalf("veio %v, queria %v", got, want)
	}
	for i := range want {
		if !got[i].Start.Equal(want[i].Start) || !got[i].End.Equal(want[i].End) {
			t.Errorf("janela %d: veio %v, queria %v", i, got[i], want[i])
		}
	}

	// janelas menores que o mínimo somem
	if got := FreeWindows(busy, h(7), h(22), 2*time.Hour); len(got) != 1 || !got[0].Start.Equal(h(16)) {
		t.Errorf("só a janela das 16h às 22h tem 2h: %v", got)
	}
	// nada ocupado: a janela inteira
	if got := FreeWindows(nil, h(7), h(22), time.Hour); len(got) != 1 || !got[0].End.Equal(h(22)) {
		t.Errorf("sem bloqueios, a janela inteira é livre: %v", got)
	}
	// tudo ocupado: nenhuma
	if got := FreeWindows([]Interval{{h(0), h(24)}}, h(7), h(22), time.Minute); len(got) != 0 {
		t.Errorf("tudo ocupado não deixa janela: %v", got)
	}
}

func TestClockAndMask(t *testing.T) {
	if m, err := ParseClock("09:30"); err != nil || m != 570 {
		t.Errorf("09:30 = %d, %v", m, err)
	}
	if m, err := ParseClock("24:00"); err != nil || m != 1440 {
		t.Errorf("24:00 = %d, %v", m, err)
	}
	for _, bad := range []string{"", "9h", "25:00", "12:60", "7"} {
		if _, err := ParseClock(bad); err == nil {
			t.Errorf("%q deveria ser inválido", bad)
		}
	}
	if FormatClock(570) != "09:30" {
		t.Error("FormatClock(570) deveria ser 09:30")
	}
	m, err := MaskFromDays([]int{7, 1, 1, 3})
	if err != nil {
		t.Fatal(err)
	}
	if d := DaysFromMask(m); len(d) != 3 || d[0] != 1 || d[1] != 3 || d[2] != 7 {
		t.Errorf("dias devolvidos %v", d)
	}
	if _, err := MaskFromDays([]int{0}); err == nil {
		t.Error("dia 0 deveria ser inválido")
	}
	if _, err := MaskFromDays([]int{8}); err == nil {
		t.Error("dia 8 deveria ser inválido")
	}
}
