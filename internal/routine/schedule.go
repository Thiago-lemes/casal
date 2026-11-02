package routine

import (
	"fmt"
	"sort"
	"time"
)

const (
	RecurrenceOnce   = "once"
	RecurrenceWeekly = "weekly"

	VisibilityPrivate   = "private"   // o parceiro não vê nada
	VisibilityBusy      = "busy"      // o parceiro vê só o horário, como "ocupado"
	VisibilityView      = "view"      // o parceiro vê os detalhes
	VisibilityHousehold = "household" // bloco do casal: os dois veem e editam
)

// Block é um bloco da rotina. Os horários são relógio de parede no fuso do dono
// (StartMin e EndMin contam minutos desde a meia-noite; EndMin 1440 é a meia-noite seguinte).
// As datas são dias do calendário, sempre à meia-noite UTC.
//
// A repetição equivale a uma regra iCalendar semanal: FREQ=WEEKLY;BYDAY=<Weekdays>;
// INTERVAL=<IntervalWeeks>;UNTIL=<EndDate>, ancorada em StartDate. Sem repetição (once), vale só em StartDate.
type Block struct {
	ID            string
	HouseholdID   string
	OwnerID       string
	OwnerName     string
	OwnerTZ       string
	Title         string
	Category      string
	Location      string
	Visibility    string
	Recurrence    string
	Weekdays      uint8 // bit 0 = segunda ... bit 6 = domingo
	IntervalWeeks int
	StartDate     time.Time
	EndDate       *time.Time
	StartMin      int
	EndMin        int
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Busy          bool // preenchido por Project: os detalhes foram escondidos
}

// Exception muda um dia específico de um bloco, sem alterar a rotina fixa.
type Exception struct {
	BlockID  string
	Date     time.Time
	Skipped  bool // folga: o bloco não acontece neste dia
	StartMin int  // horário trocado, quando não é folga
	EndMin   int
}

// Occurrence é um bloco acontecendo em um dia, já em instantes.
type Occurrence struct {
	BlockID   string
	OwnerID   string
	OwnerName string
	Title     string // vazio quando Busy
	Category  string
	Location  string
	Busy      bool
	Start     time.Time
	End       time.Time
	Exception bool // o horário vem de uma exceção
}

type Interval struct{ Start, End time.Time }

// DateKey é a chave "AAAA-MM-DD" de um dia do calendário.
func DateKey(d time.Time) string { return d.Format("2006-01-02") }

// ParseDate lê "AAAA-MM-DD" como dia do calendário (meia-noite UTC).
func ParseDate(s string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("data deve ser AAAA-MM-DD")
	}
	return d, nil
}

// ParseClock lê "HH:MM" (ou "24:00") em minutos desde a meia-noite.
func ParseClock(s string) (int, error) {
	if s == "24:00" {
		return 1440, nil
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("horário deve ser HH:MM")
	}
	return t.Hour()*60 + t.Minute(), nil
}

func FormatClock(min int) string { return fmt.Sprintf("%02d:%02d", min/60, min%60) }

// isoWeekday: segunda = 1 ... domingo = 7.
func isoWeekday(d time.Time) int { return (int(d.Weekday())+6)%7 + 1 }

func monday(d time.Time) time.Time { return d.AddDate(0, 0, -(isoWeekday(d) - 1)) }

// MaskFromDays converte dias ISO (1 a 7) em máscara; repetidos valem uma vez.
func MaskFromDays(days []int) (uint8, error) {
	var m uint8
	for _, d := range days {
		if d < 1 || d > 7 {
			return 0, fmt.Errorf("dia da semana deve ser de 1 (segunda) a 7 (domingo)")
		}
		m |= 1 << (d - 1)
	}
	return m, nil
}

func DaysFromMask(m uint8) []int {
	out := []int{}
	for d := 1; d <= 7; d++ {
		if m&(1<<(d-1)) != 0 {
			out = append(out, d)
		}
	}
	return out
}

// occursOn diz se o bloco acontece no dia (antes de qualquer exceção).
func (b Block) occursOn(d time.Time) bool {
	if d.Before(b.StartDate) || (b.EndDate != nil && d.After(*b.EndDate)) {
		return false
	}
	if b.Recurrence == RecurrenceOnce {
		return d.Equal(b.StartDate)
	}
	if b.Weekdays&(1<<(isoWeekday(d)-1)) == 0 {
		return false
	}
	interval := max(b.IntervalWeeks, 1)
	weeks := int(monday(d).Sub(monday(b.StartDate)).Hours()/24) / 7
	return weeks%interval == 0
}

func location(tz string) *time.Location {
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.UTC
}

func at(d time.Time, min int, loc *time.Location) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), min/60, min%60, 0, 0, loc)
}

// Occurrences gera as ocorrências do bloco entre os dias from e to (inclusive).
// ex indexa as exceções do bloco por DateKey.
func Occurrences(b Block, ex map[string]Exception, from, to time.Time) []Occurrence {
	loc := location(b.OwnerTZ)
	var out []Occurrence
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if !b.occursOn(d) {
			continue
		}
		start, end, changed := b.StartMin, b.EndMin, false
		if e, ok := ex[DateKey(d)]; ok {
			if e.Skipped {
				continue
			}
			start, end, changed = e.StartMin, e.EndMin, true
		}
		out = append(out, Occurrence{
			BlockID: b.ID, OwnerID: b.OwnerID, OwnerName: b.OwnerName,
			Title: b.Title, Category: b.Category, Location: b.Location, Busy: b.Busy,
			Start: at(d, start, loc), End: at(d, end, loc), Exception: changed,
		})
	}
	return out
}

// Project mostra o bloco a quem está olhando. O dono vê tudo; o parceiro vê conforme a visibilidade:
// privado não aparece, "busy" aparece sem título, categoria e local, e o resto aparece inteiro.
// A programação (dias e horários) fica intacta, para as ocorrências saírem certas.
func Project(b Block, viewer string) (Block, bool) {
	if b.OwnerID == viewer {
		return b, true
	}
	switch b.Visibility {
	case VisibilityView, VisibilityHousehold:
		return b, true
	case VisibilityBusy:
		b.Title, b.Category, b.Location, b.Busy = "", "", "", true
		return b, true
	}
	return Block{}, false
}

// CanEdit: o dono edita sempre; o parceiro só os blocos do casal.
func CanEdit(b Block, userID string) bool {
	return b.OwnerID == userID || b.Visibility == VisibilityHousehold
}

// Overlaps diz se dois blocos se sobrepõem em algum dia entre from e to (sem olhar exceções).
func Overlaps(a, b Block, from, to time.Time) bool {
	bs := Occurrences(b, nil, from, to)
	for _, oa := range Occurrences(a, nil, from, to) {
		for _, ob := range bs {
			if oa.Start.Before(ob.End) && ob.Start.Before(oa.End) {
				return true
			}
		}
	}
	return false
}

// FreeWindows devolve os intervalos livres dentro de [from, to) com pelo menos min de duração,
// dado o que está ocupado. A lista de ocupados pode ter sobreposição e estar fora de ordem.
func FreeWindows(busy []Interval, from, to time.Time, min time.Duration) []Interval {
	clipped := make([]Interval, 0, len(busy))
	for _, b := range busy {
		s, e := b.Start, b.End
		if s.Before(from) {
			s = from
		}
		if e.After(to) {
			e = to
		}
		if s.Before(e) {
			clipped = append(clipped, Interval{s, e})
		}
	}
	sort.Slice(clipped, func(i, j int) bool { return clipped[i].Start.Before(clipped[j].Start) })

	out := []Interval{}
	cursor := from
	for _, b := range clipped {
		if b.Start.After(cursor) && b.Start.Sub(cursor) >= min {
			out = append(out, Interval{cursor, b.Start})
		}
		if b.End.After(cursor) {
			cursor = b.End
		}
	}
	if to.After(cursor) && to.Sub(cursor) >= min {
		out = append(out, Interval{cursor, to})
	}
	return out
}
