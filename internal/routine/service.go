package routine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxTitle    = 120
	maxLocation = 80
	maxSpanDays = 62 // maior janela de ocorrências numa consulta
	overlapDays = 56 // quanto à frente o aviso de sobreposição olha
)

var categories = map[string]bool{
	"work": true, "study": true, "workout": true, "commute": true,
	"appointment": true, "home": true, "other": true,
}

// Person é um membro do casal, como a rotina precisa dele.
type Person struct {
	ID   string
	Name string
}

// People é o que este pacote precisa da feature household.
type People interface {
	Members(ctx context.Context, householdID string) ([]Person, error)
}

type Service struct {
	repo   Repository
	people People
	now    func() time.Time
}

func NewService(repo Repository, people People) *Service {
	return &Service{repo: repo, people: people, now: time.Now}
}

// Input é um bloco como a pessoa o escreve. Horários em "HH:MM" e datas em "AAAA-MM-DD".
type Input struct {
	Title         string
	Category      string
	Location      string
	Visibility    string // vazio: mantém (na criação, privado)
	Recurrence    string
	Weekdays      []int // 1 (segunda) a 7 (domingo)
	IntervalWeeks int   // vazio: toda semana
	StartDate     string
	EndDate       string
	Start         string
	End           string
}

// Saved é um bloco gravado com os seus avisos. Overlaps lista outros blocos do mesmo dono que se
// sobrepõem a este; é um aviso, nunca um impedimento.
type Saved struct {
	Block    Block
	Overlaps []string
}

func (s *Service) Create(ctx context.Context, householdID, userID string, in Input) (Saved, error) {
	if in.Visibility == "" {
		in.Visibility = VisibilityPrivate
	}
	b, err := build(Block{}, in)
	if err != nil {
		return Saved{}, err
	}
	b.HouseholdID, b.OwnerID = householdID, userID
	out, err := s.repo.Create(ctx, b)
	if err != nil {
		return Saved{}, err
	}
	return s.withOverlaps(ctx, householdID, userID, out)
}

func (s *Service) Get(ctx context.Context, householdID, id, viewerID string) (Block, error) {
	b, err := s.repo.Get(ctx, householdID, id, viewerID)
	if err != nil {
		return Block{}, err
	}
	p, _ := Project(b, viewerID) // o repositório já só devolve o que a pessoa vê
	return p, nil
}

// List devolve os blocos visíveis à pessoa, já com os detalhes escondidos onde só "ocupado" é permitido.
func (s *Service) List(ctx context.Context, householdID, viewerID string) ([]Block, error) {
	blocks, err := s.repo.ListVisible(ctx, householdID, viewerID)
	if err != nil {
		return nil, err
	}
	out := make([]Block, 0, len(blocks))
	for _, b := range blocks {
		if p, ok := Project(b, viewerID); ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// Update substitui o bloco. version é a que a pessoa viu; se outra gravou antes, volta ErrConflict.
func (s *Service) Update(ctx context.Context, householdID, id, userID string, in Input, version int) (Saved, error) {
	cur, err := s.repo.Get(ctx, householdID, id, userID)
	if err != nil {
		return Saved{}, err // quem não pode ver o bloco nem sabe que ele existe
	}
	if !CanEdit(cur, userID) {
		return Saved{}, ErrForbidden
	}
	if in.Visibility == "" {
		in.Visibility = cur.Visibility
	}
	if in.Visibility != cur.Visibility && cur.OwnerID != userID {
		return Saved{}, ErrForbidden // só o dono muda quem vê
	}
	b, err := build(cur, in)
	if err != nil {
		return Saved{}, err
	}
	if version != cur.Version {
		return Saved{}, ErrConflict
	}
	out, err := s.repo.Update(ctx, householdID, userID, b, version)
	if errors.Is(err, ErrNotFound) {
		return Saved{}, ErrConflict // existia e era editável um instante atrás: alguém gravou no meio
	}
	if err != nil {
		return Saved{}, err
	}
	return s.withOverlaps(ctx, householdID, userID, out)
}

// Delete: só o dono apaga. Quem vê mas não é o dono recebe ErrForbidden.
func (s *Service) Delete(ctx context.Context, householdID, id, userID string) error {
	cur, err := s.repo.Get(ctx, householdID, id, userID)
	if err != nil {
		return err
	}
	if cur.OwnerID != userID {
		return ErrForbidden
	}
	return s.repo.Delete(ctx, householdID, id, userID)
}

// SetException abre uma exceção num dia: folga (skip) ou outro horário. A rotina fixa não muda.
func (s *Service) SetException(ctx context.Context, householdID, blockID, userID, date string, skip bool, start, end string) error {
	d, err := ParseDate(date)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	cur, err := s.repo.Get(ctx, householdID, blockID, userID)
	if err != nil {
		return err
	}
	if !CanEdit(cur, userID) {
		return ErrForbidden
	}
	e := Exception{BlockID: blockID, Date: d, Skipped: skip}
	if !skip {
		if e.StartMin, e.EndMin, err = clockRange(start, end); err != nil {
			return err
		}
	} else if start != "" || end != "" {
		return fmt.Errorf("%w: folga não tem horário", ErrInvalidInput)
	}
	if !cur.occursOn(d) {
		return fmt.Errorf("%w: o bloco não acontece nesse dia", ErrInvalidInput)
	}
	return s.repo.SetException(ctx, householdID, userID, e)
}

func (s *Service) DeleteException(ctx context.Context, householdID, blockID, userID, date string) error {
	d, err := ParseDate(date)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	cur, err := s.repo.Get(ctx, householdID, blockID, userID)
	if err != nil {
		return err
	}
	if !CanEdit(cur, userID) {
		return ErrForbidden
	}
	return s.repo.DeleteException(ctx, householdID, userID, blockID, d)
}

// Occurrences devolve o que acontece entre os dias from e to (inclusive), dos dois, em ordem.
// Só entra o que a pessoa pode ver; o que é "ocupado" sai sem detalhes.
func (s *Service) Occurrences(ctx context.Context, householdID, viewerID, from, to string) ([]Occurrence, error) {
	f, err := ParseDate(from)
	if err != nil {
		return nil, fmt.Errorf("%w: from: %v", ErrInvalidInput, err)
	}
	t, err := ParseDate(to)
	if err != nil {
		return nil, fmt.Errorf("%w: to: %v", ErrInvalidInput, err)
	}
	if t.Before(f) || int(t.Sub(f).Hours()/24) > maxSpanDays {
		return nil, fmt.Errorf("%w: informe um período de até %d dias", ErrInvalidInput, maxSpanDays)
	}
	return s.occurrences(ctx, householdID, viewerID, f, t)
}

func (s *Service) occurrences(ctx context.Context, householdID, viewerID string, from, to time.Time) ([]Occurrence, error) {
	blocks, err := s.repo.ListVisible(ctx, householdID, viewerID)
	if err != nil {
		return nil, err
	}
	exceptions, err := s.repo.ListExceptions(ctx, householdID, viewerID, from, to)
	if err != nil {
		return nil, err
	}
	byBlock := map[string]map[string]Exception{}
	for _, e := range exceptions {
		if byBlock[e.BlockID] == nil {
			byBlock[e.BlockID] = map[string]Exception{}
		}
		byBlock[e.BlockID][DateKey(e.Date)] = e
	}
	out := []Occurrence{}
	for _, b := range blocks {
		p, ok := Project(b, viewerID)
		if !ok {
			continue
		}
		out = append(out, Occurrences(p, byBlock[b.ID], from, to)...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

// FreeInput descreve a busca por janelas livres. Date é um dia no fuso de quem pergunta.
type FreeInput struct {
	Date        string
	Timezone    string
	MinMinutes  int // vazio: 30
	FromHour    int
	ToHour      int
	HoursPassed bool // FromHour e ToHour foram informados (0 é uma hora válida); senão vale 7h às 22h
}

// FreeWindows devolve os intervalos do dia em que os dois estão livres ao mesmo tempo.
// Só conta o que a pessoa pode ver: o que o parceiro marcou como privado não ocupa o horário dele aqui.
func (s *Service) FreeWindows(ctx context.Context, householdID, viewerID string, in FreeInput) ([]Interval, error) {
	d, err := ParseDate(in.Date)
	if err != nil {
		return nil, fmt.Errorf("%w: date: %v", ErrInvalidInput, err)
	}
	minMinutes, fromHour, toHour := in.MinMinutes, in.FromHour, in.ToHour
	if minMinutes == 0 {
		minMinutes = 30
	}
	if !in.HoursPassed {
		fromHour, toHour = 7, 22
	}
	if minMinutes < 5 || minMinutes > 720 || fromHour < 0 || toHour > 24 || fromHour >= toHour {
		return nil, fmt.Errorf("%w: min de 5 a 720 minutos e horas de 0 a 24, com from_hour menor que to_hour", ErrInvalidInput)
	}
	loc := location(in.Timezone)
	from := time.Date(d.Year(), d.Month(), d.Day(), fromHour, 0, 0, 0, loc)
	to := time.Date(d.Year(), d.Month(), d.Day(), toHour, 0, 0, 0, loc)

	// um dia antes e um depois, porque o fuso do parceiro pode puxar um bloco para dentro da janela
	occ, err := s.occurrences(ctx, householdID, viewerID, d.AddDate(0, 0, -1), d.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	busy := make([]Interval, 0, len(occ))
	for _, o := range occ {
		busy = append(busy, Interval{o.Start, o.End})
	}
	return FreeWindows(busy, from, to, time.Duration(minMinutes)*time.Minute), nil
}

// Presence diz o que uma pessoa do casal está fazendo agora.
type Presence struct {
	UserID string
	Name   string
	Now    *Occurrence // nulo: nada marcado agora (ou nada que você possa ver)
}

// Now responde "quem está onde agora" para cada pessoa do casal.
func (s *Service) Now(ctx context.Context, householdID, viewerID string) ([]Presence, error) {
	people, err := s.people.Members(ctx, householdID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	occ, err := s.occurrences(ctx, householdID, viewerID, today.AddDate(0, 0, -1), today.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	out := make([]Presence, 0, len(people))
	for _, p := range people {
		pr := Presence{UserID: p.ID, Name: p.Name}
		for i := range occ {
			o := occ[i]
			if o.OwnerID == p.ID && !now.Before(o.Start) && now.Before(o.End) {
				pr.Now = &o
				break
			}
		}
		out = append(out, pr)
	}
	return out, nil
}

// withOverlaps avisa de sobreposição com outros blocos do mesmo dono. Só olha os do próprio dono,
// então não revela nada do parceiro.
func (s *Service) withOverlaps(ctx context.Context, householdID, ownerID string, saved Block) (Saved, error) {
	blocks, err := s.repo.ListVisible(ctx, householdID, ownerID)
	if err != nil {
		return Saved{}, err
	}
	now := s.now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, overlapDays)
	out := Saved{Block: saved, Overlaps: []string{}}
	for _, o := range blocks {
		if o.ID != saved.ID && o.OwnerID == saved.OwnerID && Overlaps(saved, o, from, to) {
			out.Overlaps = append(out.Overlaps, o.ID)
		}
	}
	return out, nil
}

// build valida a entrada e a aplica sobre base (o bloco atual, ou vazio numa criação).
func build(base Block, in Input) (Block, error) {
	b := base
	b.Title = strings.TrimSpace(in.Title)
	if b.Title == "" || utf8.RuneCountInString(b.Title) > maxTitle {
		return b, fmt.Errorf("%w: o título é obrigatório (até %d caracteres)", ErrInvalidInput, maxTitle)
	}
	if !categories[in.Category] {
		return b, fmt.Errorf("%w: categoria deve ser work, study, workout, commute, appointment, home ou other", ErrInvalidInput)
	}
	b.Category = in.Category
	b.Location = strings.TrimSpace(in.Location)
	if utf8.RuneCountInString(b.Location) > maxLocation {
		return b, fmt.Errorf("%w: o local passa de %d caracteres", ErrInvalidInput, maxLocation)
	}
	switch in.Visibility {
	case VisibilityPrivate, VisibilityBusy, VisibilityView, VisibilityHousehold:
		b.Visibility = in.Visibility
	default:
		return b, fmt.Errorf("%w: visibilidade deve ser private, busy, view ou household", ErrInvalidInput)
	}

	start, err := ParseDate(in.StartDate)
	if err != nil {
		return b, fmt.Errorf("%w: start_date: %v", ErrInvalidInput, err)
	}
	b.StartDate, b.EndDate = start, nil

	switch in.Recurrence {
	case RecurrenceOnce:
		if len(in.Weekdays) > 0 || in.EndDate != "" || (in.IntervalWeeks != 0 && in.IntervalWeeks != 1) {
			return b, fmt.Errorf("%w: bloco sem repetição não tem dias da semana, intervalo nem data final", ErrInvalidInput)
		}
		b.Recurrence, b.Weekdays, b.IntervalWeeks = RecurrenceOnce, 0, 1
	case RecurrenceWeekly:
		mask, err := MaskFromDays(in.Weekdays)
		if err != nil {
			return b, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		if mask == 0 {
			return b, fmt.Errorf("%w: escolha ao menos um dia da semana", ErrInvalidInput)
		}
		interval := in.IntervalWeeks
		if interval == 0 {
			interval = 1
		}
		if interval < 1 || interval > 4 {
			return b, fmt.Errorf("%w: o intervalo é de 1 a 4 semanas", ErrInvalidInput)
		}
		b.Recurrence, b.Weekdays, b.IntervalWeeks = RecurrenceWeekly, mask, interval
		if in.EndDate != "" {
			end, err := ParseDate(in.EndDate)
			if err != nil {
				return b, fmt.Errorf("%w: end_date: %v", ErrInvalidInput, err)
			}
			if end.Before(start) {
				return b, fmt.Errorf("%w: a data final é anterior ao início", ErrInvalidInput)
			}
			b.EndDate = &end
		}
	default:
		return b, fmt.Errorf("%w: a repetição deve ser once ou weekly", ErrInvalidInput)
	}

	if b.StartMin, b.EndMin, err = clockRange(in.Start, in.End); err != nil {
		return b, err
	}
	return b, nil
}

func clockRange(start, end string) (int, int, error) {
	s, err := ParseClock(start)
	if err != nil || s >= 1440 {
		return 0, 0, fmt.Errorf("%w: início deve ser HH:MM", ErrInvalidInput)
	}
	e, err := ParseClock(end)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: fim deve ser HH:MM (ou 24:00)", ErrInvalidInput)
	}
	if e <= s {
		return 0, 0, fmt.Errorf("%w: o fim precisa ser depois do início", ErrInvalidInput)
	}
	return s, e, nil
}
