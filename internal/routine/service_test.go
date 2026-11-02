package routine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeRepo guarda um bloco só, o bastante para exercitar as regras do service.
type fakeRepo struct {
	Repository
	block      Block
	updated    int
	exceptions []Exception
}

func (f *fakeRepo) Create(_ context.Context, b Block) (Block, error) {
	b.ID, b.Version, b.OwnerName, b.OwnerTZ = "b1", 1, "Ana", "America/Sao_Paulo"
	f.block = b
	return b, nil
}

func (f *fakeRepo) Get(_ context.Context, _, _, viewer string) (Block, error) {
	if f.block.ID == "" {
		return Block{}, ErrNotFound
	}
	if _, ok := Project(f.block, viewer); !ok {
		return Block{}, ErrNotFound
	}
	return f.block, nil
}

func (f *fakeRepo) ListVisible(_ context.Context, _, viewer string) ([]Block, error) {
	if f.block.ID == "" {
		return nil, nil
	}
	if _, ok := Project(f.block, viewer); !ok {
		return nil, nil
	}
	return []Block{f.block}, nil
}

func (f *fakeRepo) Update(_ context.Context, _, _ string, b Block, version int) (Block, error) {
	if version != f.block.Version {
		return Block{}, ErrNotFound
	}
	b.Version = version + 1
	f.block = b
	f.updated++
	return b, nil
}

func (f *fakeRepo) Delete(context.Context, string, string, string) error {
	f.block = Block{}
	return nil
}

func (f *fakeRepo) ListExceptions(context.Context, string, string, time.Time, time.Time) ([]Exception, error) {
	return f.exceptions, nil
}

func (f *fakeRepo) SetException(_ context.Context, _, _ string, e Exception) error {
	f.exceptions = append(f.exceptions, e)
	return nil
}

type fakePeople struct{}

func (fakePeople) Members(context.Context, string) ([]Person, error) {
	return []Person{{ID: "ana", Name: "Ana"}, {ID: "beto", Name: "Beto"}}, nil
}

// 2026-11-02, segunda-feira, 12:00 em São Paulo.
var fixedNow = time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)

func newService(b Block) (*Service, *fakeRepo) {
	repo := &fakeRepo{block: b}
	svc := NewService(repo, fakePeople{})
	svc.now = func() time.Time { return fixedNow }
	return svc, repo
}

func valid() Input {
	return Input{
		Title: "Trabalho", Category: "work", Recurrence: RecurrenceWeekly, Weekdays: []int{1, 2, 3, 4, 5},
		StartDate: "2026-11-02", Start: "09:00", End: "18:00",
	}
}

func TestCreateDefaultsAndCleans(t *testing.T) {
	svc, _ := newService(Block{})
	in := valid()
	in.Title, in.Location, in.IntervalWeeks = "  Trabalho ", " escritório ", 0
	got, err := svc.Create(context.Background(), "h1", "ana", in)
	if err != nil {
		t.Fatal(err)
	}
	b := got.Block
	if b.Visibility != VisibilityPrivate || b.Title != "Trabalho" || b.Location != "escritório" || b.IntervalWeeks != 1 {
		t.Errorf("padrões ou limpeza errados: %+v", b)
	}
	if b.StartMin != 540 || b.EndMin != 1080 || len(DaysFromMask(b.Weekdays)) != 5 || b.OwnerID != "ana" {
		t.Errorf("horários ou dias errados: %+v", b)
	}
}

func TestCreateValidation(t *testing.T) {
	mod := func(f func(*Input)) Input { in := valid(); f(&in); return in }
	cases := []struct {
		name string
		in   Input
	}{
		{"sem título", mod(func(i *Input) { i.Title = " " })},
		{"título longo", mod(func(i *Input) { i.Title = strings.Repeat("a", 121) })},
		{"categoria inválida", mod(func(i *Input) { i.Category = "lazer" })},
		{"local longo", mod(func(i *Input) { i.Location = strings.Repeat("a", 81) })},
		{"visibilidade inválida", mod(func(i *Input) { i.Visibility = "todos" })},
		{"repetição inválida", mod(func(i *Input) { i.Recurrence = "daily" })},
		{"semanal sem dias", mod(func(i *Input) { i.Weekdays = nil })},
		{"dia inválido", mod(func(i *Input) { i.Weekdays = []int{8} })},
		{"intervalo demais", mod(func(i *Input) { i.IntervalWeeks = 5 })},
		{"intervalo negativo", mod(func(i *Input) { i.IntervalWeeks = -1 })},
		{"sem data", mod(func(i *Input) { i.StartDate = "" })},
		{"data torta", mod(func(i *Input) { i.StartDate = "02/11/2026" })},
		{"fim antes do início da regra", mod(func(i *Input) { i.EndDate = "2026-10-01" })},
		{"data final torta", mod(func(i *Input) { i.EndDate = "ontem" })},
		{"sem hora", mod(func(i *Input) { i.Start = "" })},
		{"fim antes do início", mod(func(i *Input) { i.Start, i.End = "18:00", "09:00" })},
		{"fim igual ao início", mod(func(i *Input) { i.Start, i.End = "09:00", "09:00" })},
		{"início 24:00", mod(func(i *Input) { i.Start, i.End = "24:00", "24:00" })},
		{"único com dias", mod(func(i *Input) { i.Recurrence = RecurrenceOnce })},
		{"único com data final", mod(func(i *Input) { i.Recurrence, i.Weekdays, i.EndDate = RecurrenceOnce, nil, "2026-12-01" })},
		{"único com intervalo", mod(func(i *Input) { i.Recurrence, i.Weekdays, i.IntervalWeeks = RecurrenceOnce, nil, 2 })},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo := newService(Block{})
			if _, err := svc.Create(context.Background(), "h1", "ana", c.in); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("esperava ErrInvalidInput, veio %v", err)
			}
			if repo.block.ID != "" {
				t.Error("não deveria ter gravado")
			}
		})
	}
}

func TestCreateOnceAndEndsAtMidnight(t *testing.T) {
	svc, _ := newService(Block{})
	in := Input{Title: "Festa", Category: "other", Recurrence: RecurrenceOnce, StartDate: "2026-11-07", Start: "20:00", End: "24:00"}
	got, err := svc.Create(context.Background(), "h1", "ana", in)
	if err != nil || got.Block.EndMin != 1440 || got.Block.Weekdays != 0 || got.Block.IntervalWeeks != 1 {
		t.Errorf("bloco único mal montado: %+v %v", got.Block, err)
	}
}

func TestOverlapWarningListsOnlyOwnBlocks(t *testing.T) {
	// o bloco do parceiro (visível) cobre o mesmo horário, mas não gera aviso para a Ana
	partner := Block{
		ID: "b9", OwnerID: "beto", OwnerTZ: "America/Sao_Paulo", Visibility: VisibilityView,
		Recurrence: RecurrenceWeekly, Weekdays: 0b0000001, IntervalWeeks: 1, StartDate: day("2026-11-02"),
		StartMin: 9 * 60, EndMin: 18 * 60,
	}
	svc, _ := newService(partner)
	saved := partner
	saved.ID, saved.OwnerID = "b1", "ana"
	got, err := svc.withOverlaps(context.Background(), "h1", "ana", saved)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Overlaps) != 0 {
		t.Errorf("o aviso não pode citar blocos do parceiro: %v", got.Overlaps)
	}
	own := partner
	own.OwnerID = "ana"
	svc, _ = newService(own)
	got, err = svc.withOverlaps(context.Background(), "h1", "ana", saved)
	if err != nil || len(got.Overlaps) != 1 || got.Overlaps[0] != "b9" {
		t.Errorf("deveria avisar do bloco da própria Ana: %v %v", got.Overlaps, err)
	}
}

func TestCanEdit(t *testing.T) {
	cases := []struct {
		visibility, user string
		want             bool
	}{
		{VisibilityPrivate, "ana", true},
		{VisibilityHousehold, "ana", true},
		{VisibilityPrivate, "beto", false},
		{VisibilityBusy, "beto", false},
		{VisibilityView, "beto", false},
		{VisibilityHousehold, "beto", true},
	}
	for _, c := range cases {
		if got := CanEdit(Block{OwnerID: "ana", Visibility: c.visibility}, c.user); got != c.want {
			t.Errorf("CanEdit(%s, %s) = %v, queria %v", c.visibility, c.user, got, c.want)
		}
	}
}

func stored(visibility string) Block {
	return Block{
		ID: "b1", OwnerID: "ana", OwnerName: "Ana", OwnerTZ: "America/Sao_Paulo", Title: "Trabalho",
		Category: "work", Visibility: visibility, Recurrence: RecurrenceWeekly, Weekdays: 0b0011111,
		IntervalWeeks: 1, StartDate: day("2026-11-02"), StartMin: 540, EndMin: 1080, Version: 1,
	}
}

func TestUpdatePermissions(t *testing.T) {
	ctx := context.Background()
	t.Run("bloco privado: o parceiro nem sabe que existe", func(t *testing.T) {
		svc, _ := newService(stored(VisibilityPrivate))
		if _, err := svc.Update(ctx, "h1", "b1", "beto", valid(), 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("esperava ErrNotFound, veio %v", err)
		}
	})
	t.Run("ocupado ou só leitura: vê mas não edita", func(t *testing.T) {
		for _, v := range []string{VisibilityBusy, VisibilityView} {
			svc, repo := newService(stored(v))
			if _, err := svc.Update(ctx, "h1", "b1", "beto", valid(), 1); !errors.Is(err, ErrForbidden) {
				t.Errorf("%s: esperava ErrForbidden, veio %v", v, err)
			}
			if repo.updated != 0 {
				t.Errorf("%s: não deveria ter gravado", v)
			}
		}
	})
	t.Run("bloco do casal: o parceiro edita mas não muda a visibilidade", func(t *testing.T) {
		svc, _ := newService(stored(VisibilityHousehold))
		if _, err := svc.Update(ctx, "h1", "b1", "beto", valid(), 1); err != nil {
			t.Errorf("o parceiro deveria editar: %v", err)
		}
		hide := valid()
		hide.Visibility = VisibilityPrivate
		if _, err := svc.Update(ctx, "h1", "b1", "beto", hide, 2); !errors.Is(err, ErrForbidden) {
			t.Errorf("o parceiro não pode esconder o bloco: %v", err)
		}
	})
	t.Run("o dono muda a visibilidade; vazio mantém", func(t *testing.T) {
		svc, _ := newService(stored(VisibilityPrivate))
		share := valid()
		share.Visibility = VisibilityBusy
		got, err := svc.Update(ctx, "h1", "b1", "ana", share, 1)
		if err != nil || got.Block.Visibility != VisibilityBusy {
			t.Fatalf("visibilidade não mudou: %+v %v", got.Block, err)
		}
		keep, err := svc.Update(ctx, "h1", "b1", "ana", valid(), 2)
		if err != nil || keep.Block.Visibility != VisibilityBusy {
			t.Errorf("visibilidade vazia deveria manter busy: %+v %v", keep.Block, err)
		}
	})
	t.Run("versão velha dá conflito e não grava", func(t *testing.T) {
		b := stored(VisibilityHousehold)
		b.Version = 3
		svc, repo := newService(b)
		if _, err := svc.Update(ctx, "h1", "b1", "ana", valid(), 2); !errors.Is(err, ErrConflict) {
			t.Errorf("esperava ErrConflict, veio %v", err)
		}
		if repo.updated != 0 {
			t.Error("versão velha não pode gravar")
		}
	})
}

func TestDeleteOnlyOwner(t *testing.T) {
	svc, repo := newService(stored(VisibilityHousehold))
	if err := svc.Delete(context.Background(), "h1", "b1", "beto"); !errors.Is(err, ErrForbidden) {
		t.Errorf("o parceiro não apaga, nem bloco do casal: %v", err)
	}
	if repo.block.ID == "" {
		t.Fatal("o bloco não deveria ter sido apagado")
	}
	if err := svc.Delete(context.Background(), "h1", "b1", "ana"); err != nil {
		t.Errorf("o dono deveria apagar: %v", err)
	}
}

func TestSetException(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(stored(VisibilityView))

	// 2026-11-03 é terça: o bloco acontece (seg a sex)
	if err := svc.SetException(ctx, "h1", "b1", "ana", "2026-11-03", true, "", ""); err != nil {
		t.Fatalf("folga deveria valer: %v", err)
	}
	if err := svc.SetException(ctx, "h1", "b1", "ana", "2026-11-04", false, "13:00", "17:00"); err != nil {
		t.Fatalf("horário trocado deveria valer: %v", err)
	}
	if len(repo.exceptions) != 2 || !repo.exceptions[0].Skipped || repo.exceptions[1].StartMin != 780 {
		t.Errorf("exceções mal gravadas: %+v", repo.exceptions)
	}

	bad := []struct {
		name       string
		user, date string
		skip       bool
		start, end string
		want       error
	}{
		{"dia em que o bloco não acontece (sábado)", "ana", "2026-11-07", true, "", "", ErrInvalidInput},
		{"antes do início da regra", "ana", "2026-10-26", true, "", "", ErrInvalidInput},
		{"data torta", "ana", "hoje", true, "", "", ErrInvalidInput},
		{"horário trocado sem horário", "ana", "2026-11-05", false, "", "", ErrInvalidInput},
		{"horário trocado invertido", "ana", "2026-11-05", false, "17:00", "13:00", ErrInvalidInput},
		{"folga com horário", "ana", "2026-11-05", true, "09:00", "10:00", ErrInvalidInput},
		{"parceiro de bloco só de leitura", "beto", "2026-11-05", true, "", "", ErrForbidden},
	}
	for _, c := range bad {
		if err := svc.SetException(ctx, "h1", "b1", c.user, c.date, c.skip, c.start, c.end); !errors.Is(err, c.want) {
			t.Errorf("%s: esperava %v, veio %v", c.name, c.want, err)
		}
	}
	if len(repo.exceptions) != 2 {
		t.Errorf("exceção inválida não pode gravar: %+v", repo.exceptions)
	}

	// bloco do casal: o parceiro abre exceção
	svc, _ = newService(stored(VisibilityHousehold))
	if err := svc.SetException(ctx, "h1", "b1", "beto", "2026-11-05", true, "", ""); err != nil {
		t.Errorf("o parceiro deveria abrir exceção num bloco do casal: %v", err)
	}
}

func TestOccurrencesWindowLimits(t *testing.T) {
	svc, _ := newService(stored(VisibilityView))
	ctx := context.Background()
	for _, c := range []struct{ name, from, to string }{
		{"período invertido", "2026-11-10", "2026-11-01"},
		{"período longo demais", "2026-01-01", "2026-12-31"},
		{"data torta", "x", "2026-11-01"},
		{"sem fim", "2026-11-01", ""},
	} {
		if _, err := svc.Occurrences(ctx, "h1", "ana", c.from, c.to); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: esperava ErrInvalidInput, veio %v", c.name, err)
		}
	}
	got, err := svc.Occurrences(ctx, "h1", "ana", "2026-11-02", "2026-11-08")
	if err != nil || len(got) != 5 {
		t.Errorf("uma semana de seg a sex deveria ter 5 ocorrências: %d %v", len(got), err)
	}
}

func TestOccurrencesHideWhatYouCannotSee(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(stored(VisibilityPrivate))
	got, err := svc.Occurrences(ctx, "h1", "beto", "2026-11-02", "2026-11-08")
	if err != nil || len(got) != 0 {
		t.Errorf("o bloco privado da Ana não pode aparecer para o Beto: %v %v", got, err)
	}
	svc, _ = newService(stored(VisibilityBusy))
	got, err = svc.Occurrences(ctx, "h1", "beto", "2026-11-02", "2026-11-02")
	if err != nil || len(got) != 1 || !got[0].Busy || got[0].Title != "" || got[0].Category != "" {
		t.Errorf("o Beto deveria ver só 'ocupado': %+v %v", got, err)
	}
}

func TestFreeWindowsValidationAndPrivacy(t *testing.T) {
	ctx := context.Background()
	in := FreeInput{Date: "2026-11-02", Timezone: "America/Sao_Paulo"}

	for _, c := range []struct {
		name string
		in   FreeInput
	}{
		{"data torta", FreeInput{Date: "x"}},
		{"min pequeno", FreeInput{Date: "2026-11-02", MinMinutes: 1}},
		{"min grande", FreeInput{Date: "2026-11-02", MinMinutes: 721}},
		{"horas invertidas", FreeInput{Date: "2026-11-02", FromHour: 20, ToHour: 8, HoursPassed: true}},
		{"hora fora do dia", FreeInput{Date: "2026-11-02", FromHour: 0, ToHour: 25, HoursPassed: true}},
	} {
		svc, _ := newService(Block{})
		if _, err := svc.FreeWindows(ctx, "h1", "ana", c.in); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: esperava ErrInvalidInput, veio %v", c.name, err)
		}
	}

	// segunda 9h às 18h em São Paulo: das 7h às 22h sobram 7h-9h e 18h-22h
	svc, _ := newService(stored(VisibilityView))
	got, err := svc.FreeWindows(ctx, "h1", "beto", in)
	if err != nil || len(got) != 2 {
		t.Fatalf("esperava 2 janelas, veio %v %v", got, err)
	}
	sp := location("America/Sao_Paulo")
	if got[0].Start.In(sp).Hour() != 7 || got[0].End.In(sp).Hour() != 9 || got[1].Start.In(sp).Hour() != 18 || got[1].End.In(sp).Hour() != 22 {
		t.Errorf("janelas erradas: %v", got)
	}

	// o que o parceiro marcou como privado não ocupa o horário dele na conta do outro
	svc, _ = newService(stored(VisibilityPrivate))
	got, err = svc.FreeWindows(ctx, "h1", "beto", in)
	if err != nil || len(got) != 1 || got[0].End.Sub(got[0].Start) != 15*time.Hour {
		t.Errorf("com o bloco privado, o Beto deveria ver o dia todo livre: %v %v", got, err)
	}
	// mas para a própria Ana ele conta
	got, err = svc.FreeWindows(ctx, "h1", "ana", in)
	if err != nil || len(got) != 2 {
		t.Errorf("para a dona, o bloco privado ocupa o horário: %v %v", got, err)
	}

	// janela escolhida e mínimo
	in.HoursPassed, in.FromHour, in.ToHour, in.MinMinutes = true, 8, 20, 180
	svc, _ = newService(stored(VisibilityView))
	got, err = svc.FreeWindows(ctx, "h1", "ana", in)
	if err != nil || len(got) != 0 {
		t.Errorf("com mínimo de 3h nenhuma janela (8h-9h e 18h-20h são menores): %v %v", got, err)
	}
}

func TestNowShowsWhoIsWhere(t *testing.T) {
	ctx := context.Background()
	// fixedNow = segunda 12:00 em São Paulo; o bloco da Ana é 9h às 18h
	svc, _ := newService(stored(VisibilityView))
	got, err := svc.Now(ctx, "h1", "beto")
	if err != nil || len(got) != 2 {
		t.Fatalf("esperava os 2 do casal: %v %v", got, err)
	}
	if got[0].UserID != "ana" || got[0].Now == nil || got[0].Now.Title != "Trabalho" {
		t.Errorf("a Ana deveria estar no trabalho: %+v", got[0])
	}
	if got[1].UserID != "beto" || got[1].Now != nil {
		t.Errorf("o Beto não tem nada agora: %+v", got[1])
	}

	svc, _ = newService(stored(VisibilityBusy))
	got, _ = svc.Now(ctx, "h1", "beto")
	if got[0].Now == nil || !got[0].Now.Busy || got[0].Now.Title != "" {
		t.Errorf("o Beto deveria ver a Ana só como ocupada: %+v", got[0])
	}

	svc, _ = newService(stored(VisibilityPrivate))
	got, _ = svc.Now(ctx, "h1", "beto")
	if got[0].Now != nil {
		t.Errorf("o Beto não pode ver o bloco privado da Ana: %+v", got[0])
	}

	// depois do expediente
	svc, _ = newService(stored(VisibilityView))
	svc.now = func() time.Time { return time.Date(2026, 11, 2, 22, 0, 0, 0, time.UTC) } // 19h em São Paulo
	got, _ = svc.Now(ctx, "h1", "beto")
	if got[0].Now != nil {
		t.Errorf("às 19h a Ana já saiu do trabalho: %+v", got[0])
	}
}
