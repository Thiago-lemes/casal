package task

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"casal/internal/testutil"
)

func refEnv(t *testing.T) (*Service, string, string, string) {
	t.Helper()
	db := testutil.OpenDB(t)
	ana := testutil.NewUser(t, db, "ana")
	beto := testutil.NewUser(t, db, "beto")
	hh := testutil.NewHousehold(t, db, ana, beto)
	svc := NewService(NewPostgresRepo(db), fakeMembers{ok: true}, &fakeNotifier{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, hh, ana, beto
}

func TestCreateWithRefIsIdempotentAndCancelFreesTheRef(t *testing.T) {
	svc, hh, ana, _ := refEnv(t)
	ctx := context.Background()

	first, err := svc.CreateWithRef(ctx, hh, ana, RefInput{Title: "Comprar tinta"}, "goal:g1:step:s1")
	if err != nil || first.Ref == nil || *first.Ref != "goal:g1:step:s1" || first.AssigneeID != nil {
		t.Fatalf("criar: %+v %v", first, err)
	}
	again, err := svc.CreateWithRef(ctx, hh, ana, RefInput{Title: "outro título"}, "goal:g1:step:s1")
	if err != nil || again.ID != first.ID {
		t.Fatalf("o mesmo ref deveria devolver a mesma tarefa: %+v %v", again, err)
	}

	// duas criações ao mesmo tempo também resultam em uma tarefa só
	var wg sync.WaitGroup
	ids := make([]string, 6)
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tk, err := svc.CreateWithRef(ctx, hh, ana, RefInput{Title: "paralela"}, "goal:g1:step:s2")
			if err != nil {
				t.Error(err)
			}
			ids[i] = tk.ID
		}()
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("criações simultâneas geraram tarefas diferentes: %v", ids)
		}
	}

	n, err := svc.CancelByRef(ctx, hh, "goal:g1:step:s1")
	if err != nil || n != 1 {
		t.Fatalf("cancelar: %d %v", n, err)
	}
	if n, err := svc.CancelByRef(ctx, hh, "goal:g1:step:s1"); err != nil || n != 0 {
		t.Errorf("cancelar de novo não é erro nem cancela nada: %d %v", n, err)
	}
	// cancelada, o ref fica livre para uma tarefa nova
	fresh, err := svc.CreateWithRef(ctx, hh, ana, RefInput{Title: "Comprar tinta"}, "goal:g1:step:s1")
	if err != nil || fresh.ID == first.ID {
		t.Fatalf("depois de cancelar, o ref deveria gerar tarefa nova: %+v %v", fresh, err)
	}
}

func TestStatesByRef(t *testing.T) {
	svc, hh, ana, beto := refEnv(t)
	ctx := context.Background()
	mk := func(ref, assignee string) Task {
		tk, err := svc.CreateWithRef(ctx, hh, ana, RefInput{Title: "t " + ref, AssigneeID: assignee}, ref)
		if err != nil {
			t.Fatal(err)
		}
		return tk
	}
	open, done, canceled := mk("r-open", beto), mk("r-done", ""), mk("r-canceled", "")
	if _, err := svc.Complete(ctx, hh, done.ID, ana); err != nil {
		t.Fatal(err)
	}
	if err := svc.Cancel(ctx, hh, canceled.ID, ana); err != nil {
		t.Fatal(err)
	}

	got, err := svc.StatesByRef(ctx, hh, []string{"r-open", "r-done", "r-canceled", "r-none"})
	if err != nil {
		t.Fatal(err)
	}
	if st := got["r-open"]; st.Status != StatusOpen || st.AssigneeID == nil || *st.AssigneeID != beto || st.TaskID != open.ID {
		t.Errorf("aberta: %+v", st)
	}
	if st := got["r-done"]; st.Status != StatusDone || st.CompletedAt == nil {
		t.Errorf("concluída: %+v", st)
	}
	if st := got["r-canceled"]; st.Status != StatusCanceled {
		t.Errorf("cancelada: %+v", st)
	}
	if _, ok := got["r-none"]; ok {
		t.Error("ref sem tarefa não aparece")
	}

	// a tarefa nova do mesmo ref vence a cancelada antiga
	again := mk("r-canceled", "")
	got, _ = svc.StatesByRef(ctx, hh, []string{"r-canceled"})
	if got["r-canceled"].Status != StatusOpen || got["r-canceled"].TaskID != again.ID {
		t.Errorf("a ativa deveria vencer a cancelada: %+v", got["r-canceled"])
	}
}

func TestCancelByRefKeepsDoneTasksAsHistory(t *testing.T) {
	svc, hh, ana, _ := refEnv(t)
	ctx := context.Background()
	tk, _ := svc.CreateWithRef(ctx, hh, ana, RefInput{Title: "feita"}, "goal:g:step:done")
	if _, err := svc.Complete(ctx, hh, tk.ID, ana); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.CancelByRef(ctx, hh, "goal:g:step:done"); err != nil || n != 0 {
		t.Fatalf("a concluída não é cancelada: %d %v", n, err)
	}
	got, _ := svc.List(ctx, hh, StatusDone)
	found := false
	for _, x := range got {
		if x.ID == tk.ID {
			found = true
			if x.Ref != nil {
				t.Errorf("a concluída deveria perder o vínculo: %v", *x.Ref)
			}
		}
	}
	if !found {
		t.Error("a tarefa concluída deveria continuar como histórico")
	}
}

func TestAfterChangeRunsOutsideAndNeverBreaksTheOperation(t *testing.T) {
	svc, hh, ana, _ := refEnv(t)
	ctx := context.Background()
	var mu sync.Mutex
	var seen []string
	svc.AfterChange(func(_ context.Context, tk Task) {
		mu.Lock()
		defer mu.Unlock()
		ref := ""
		if tk.Ref != nil {
			ref = *tk.Ref
		}
		seen = append(seen, tk.Status+":"+ref)
	})
	// um gancho que quebra não derruba a conclusão nem impede os seguintes
	svc.AfterChange(func(context.Context, Task) { panic("gancho quebrado") })

	a, _ := svc.CreateWithRef(ctx, hh, ana, RefInput{Title: "a"}, "goal:g:step:a")
	b, _ := svc.CreateWithRef(ctx, hh, ana, RefInput{Title: "b"}, "goal:g:step:b")
	if _, err := svc.Complete(ctx, hh, a.ID, ana); err != nil {
		t.Fatal(err)
	}
	if err := svc.Cancel(ctx, hh, b.ID, ana); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 || seen[0] != "done:goal:g:step:a" || seen[1] != "canceled:goal:g:step:b" {
		t.Errorf("o gancho deveria ver a conclusão e o cancelamento: %v", seen)
	}
}
