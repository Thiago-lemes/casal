package goal

import (
	"testing"
	"time"
)

// segunda 16/11/2026, 07:00 em São Paulo (10:00 UTC): o lembrete das 09:00 ainda está no futuro.
var mondayMorning = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

func reminded(e *env, g Goal, day string) *fakeRem {
	return e.rem.get("goal:" + g.ID + ":" + day)
}

func (f *fakeReminders) get(ref string) *fakeRem {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.byRef[ref]; r != nil && !r.Canceled {
		return r
	}
	return nil
}

func (e *env) runReminderJob() {
	e.t.Helper()
	_, err := e.svc.ReconcileHabitReminders(ctx())
	mustNoErr(e.t, err)
}

func TestHabitReminderIsCreatedOncePerDayAtTheOwnersLocalTime(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.now = mondayMorning
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3, RemindEnabled: true}) // 09:00 padrão

	ref := "goal:" + g.ID + ":2026-11-16"
	r := e.rem.get(ref)
	if r == nil {
		t.Fatalf("o lembrete do dia deveria existir ao criar: %+v", e.rem.active())
	}
	if want := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC); !r.At.Equal(want) { // 09:00 em São Paulo
		t.Errorf("horário %v, quero %v", r.At, want)
	}
	if r.Recipient != e.ana.UserID || r.Body != "Hábito: Correr" {
		t.Errorf("lembrete: %+v", r)
	}

	// rodar o job várias vezes não duplica
	e.runReminderJob()
	e.runReminderJob()
	if e.rem.creates != 1 || len(e.rem.active()) != 1 {
		t.Errorf("criações=%d ativos=%d", e.rem.creates, len(e.rem.active()))
	}
}

func TestHabitReminderUsesTheDayOfTheOwnerNotOfTheServer(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.people.ps[0].Timezone = "Asia/Tokyo"
	// 20:00 UTC de 15/11 = 05:00 de 16/11 em Tóquio; o lembrete das 07:00 de lá é 22:00 UTC de 15/11
	e.now = time.Date(2026, 11, 15, 20, 0, 0, 0, time.UTC)
	remind := 7 * 60
	g := e.habit(e.ana, "Alongar", HabitInput{TargetCount: 3, RemindEnabled: true, RemindMin: &remind})
	r := e.rem.get("goal:" + g.ID + ":2026-11-16")
	if r == nil || !r.At.Equal(time.Date(2026, 11, 15, 22, 0, 0, 0, time.UTC)) {
		t.Fatalf("lembrete no fuso do dono: %+v (ativos %v)", r, e.rem.active())
	}
}

func TestHabitReminderIsSkippedWhenItNoLongerMatters(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.now = mondayMorning

	// 1. fora dos dias combinados (só terça)
	tue := e.habit(e.ana, "Só terça", HabitInput{TargetCount: 1, Weekdays: 2, RemindEnabled: true})
	if reminded(e, tue, "2026-11-16") != nil {
		t.Error("segunda não está nos dias combinados")
	}
	// 2. o horário de hoje já passou
	e.now = time.Date(2026, 11, 16, 20, 0, 0, 0, time.UTC) // 17h em São Paulo
	late := e.habit(e.ana, "Tarde demais", HabitInput{TargetCount: 3, RemindEnabled: true})
	if len(e.rem.active()) != 0 {
		t.Errorf("horário passado não cria lembrete: %v", e.rem.active())
	}
	_ = late

	// 3. lembrete desligado
	e.now = mondayMorning
	e.habit(e.ana, "Sem lembrete", HabitInput{TargetCount: 3})
	if len(e.rem.active()) != 0 {
		t.Errorf("sem remind_enabled: %v", e.rem.active())
	}

	// 4. já cumpriu o período: marcou a meta inteira na semana (1 por semana, feito ontem)
	one := e.habit(e.ana, "Uma por semana", HabitInput{TargetCount: 1, Period: PeriodWeek})
	e.now = time.Date(2026, 11, 15, 15, 0, 0, 0, time.UTC) // domingo
	e.mark(e.ana, one.ID, "")
	e.now = mondayMorning // segunda: semana nova, meta de novo em aberto
	cur, _ := e.svc.Get(ctx(), e.ana, one.ID)
	if cur.Progress.Done != 0 {
		t.Fatalf("semana nova: %+v", cur.Progress)
	}
	// terça: marcou na segunda, então a meta da semana está cumprida
	e.mark(e.ana, one.ID, "")
	patch := 9 * 60
	_, err := e.svc.Update(ctx(), e.ana, one.ID, UpdateInput{Version: cur.Version, Title: one.Title,
		Habit: &struct {
			TargetCount   int  `json:"target_count"`
			Weekdays      int  `json:"weekdays"`
			RemindEnabled bool `json:"remind_enabled"`
			RemindMin     *int `json:"remind_min"`
		}{TargetCount: 1, RemindEnabled: true, RemindMin: &patch}})
	mustNoErr(t, err)
	e.now = time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	e.runReminderJob()
	if reminded(e, one, "2026-11-17") != nil {
		t.Error("quem já cumpriu o período não recebe lembrete")
	}
}

func TestMarkingTheDayCancelsItsReminder(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.now = mondayMorning
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3, RemindEnabled: true})
	if len(e.rem.active()) != 1 {
		t.Fatalf("lembrete criado: %v", e.rem.active())
	}
	e.mark(e.ana, g.ID, "")
	if len(e.rem.active()) != 0 {
		t.Errorf("marcar cancela o lembrete do dia: %v", e.rem.active())
	}
	e.runReminderJob() // o job não o recria
	if len(e.rem.active()) != 0 {
		t.Errorf("o job recriou o lembrete de um dia marcado: %v", e.rem.active())
	}
	// desmarcou: o lembrete volta (o horário ainda não passou)
	_, err := e.svc.UndoCheckin(ctx(), e.ana, g.ID, "2026-11-16")
	mustNoErr(t, err)
	if len(e.rem.active()) != 1 {
		t.Errorf("desmarcar traz o lembrete de volta: %v", e.rem.active())
	}
}

func TestPausingCompletingAndDeletingCancelTheReminder(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.now = mondayMorning
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3, RemindEnabled: true})

	_, err := e.svc.Pause(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	e.runReminderJob()
	if len(e.rem.active()) != 0 {
		t.Errorf("pausar cancela e o job respeita: %v", e.rem.active())
	}
	_, err = e.svc.Resume(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	if len(e.rem.active()) != 1 {
		t.Errorf("retomar refaz o lembrete de hoje: %v", e.rem.active())
	}
	_, err = e.svc.Complete(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	if len(e.rem.active()) != 0 {
		t.Errorf("concluir cancela: %v", e.rem.active())
	}
	_, _ = e.svc.Resume(ctx(), e.ana, g.ID)
	mustNoErr(t, e.svc.Delete(ctx(), e.ana, g.ID))
	if len(e.rem.active()) != 0 {
		t.Errorf("apagar cancela: %v", e.rem.active())
	}
}

func TestChangingTheReminderTimeRebuildsTodaysReminder(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.now = mondayMorning
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3, RemindEnabled: true})
	remind := 8*60 + 30
	_, err := e.svc.Update(ctx(), e.ana, g.ID, UpdateInput{Version: g.Version, Title: g.Title,
		Habit: &struct {
			TargetCount   int  `json:"target_count"`
			Weekdays      int  `json:"weekdays"`
			RemindEnabled bool `json:"remind_enabled"`
			RemindMin     *int `json:"remind_min"`
		}{TargetCount: 3, RemindEnabled: true, RemindMin: &remind}})
	mustNoErr(t, err)
	r := e.rem.get("goal:" + g.ID + ":2026-11-16")
	if r == nil || !r.At.Equal(time.Date(2026, 11, 16, 11, 30, 0, 0, time.UTC)) {
		t.Errorf("novo horário 08:30: %+v", r)
	}
}

func TestReminderJobOnlyLooksAtHouseholdsWithTheModuleOn(t *testing.T) {
	e := newEnv(t)
	e.now = mondayMorning
	// módulo sem linha em household_modules: o job não enxerga o casal
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3, RemindEnabled: true})
	e.rem.CancelByRef(ctx(), e.hh, "goal:"+g.ID+":2026-11-16")
	e.runReminderJob()
	if len(e.rem.active()) != 0 {
		t.Fatalf("módulo desligado: o job não cria lembrete: %v", e.rem.active())
	}
	e.enableModule()
	e.runReminderJob()
	if len(e.rem.active()) != 1 {
		t.Errorf("módulo ligado: o job cria: %v", e.rem.active())
	}

	// desligar o módulo cancela o que existe; religar refaz
	e.svc.ModuleChanged(ctx(), e.hh, false)
	if len(e.rem.active()) != 0 {
		t.Errorf("desligar cancela os lembretes: %v", e.rem.active())
	}
	e.svc.ModuleChanged(ctx(), e.hh, true)
	if len(e.rem.active()) != 1 {
		t.Errorf("religar refaz os lembretes: %v", e.rem.active())
	}
}

func TestReminderFailureDoesNotBreakTheGoal(t *testing.T) {
	e := newEnv(t)
	e.now = mondayMorning
	e.rem.failCreate = true
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3, RemindEnabled: true})
	if g.ID == "" || len(e.rem.active()) != 0 {
		t.Fatalf("a meta existe mesmo sem o lembrete: %+v", g)
	}
	e.enableModule()
	e.rem.failCreate = false
	e.runReminderJob() // o job refaz por recálculo
	if len(e.rem.active()) != 1 {
		t.Errorf("o job deveria refazer: %v", e.rem.active())
	}
}
