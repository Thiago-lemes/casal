package goal

import (
	"time"

	"casal/internal/apperr"
)

const (
	KindFinancial = "financial"
	KindHabit     = "habit"
	KindShared    = "shared"
	KindProject   = "project"

	ScopeHousehold = "household"
	ScopePersonal  = "personal"

	VisibilityHousehold = "household"
	VisibilityPrivate   = "private" // só o dono
	VisibilityView      = "view"    // o parceiro lê título, prazo e progresso

	StatusActive   = "active"
	StatusPaused   = "paused"
	StatusDone     = "done"
	StatusArchived = "archived"

	SourceContributions = "contributions"
	SourceBudgetSavings = "budget_savings"

	PeriodWeek  = "week"
	PeriodMonth = "month"

	// AccessFull: dono, ou meta do casal. AccessView: o parceiro lendo uma meta pessoal "view".
	AccessFull = "full"
	AccessView = "view"

	MaxAmountCents   = 1_000_000_000 // R$ 10.000.000,00, o mesmo teto de Finanças
	MaxActiveGoals   = 100
	MaxSteps         = 200
	MaxBackfillDays  = 31
	DefaultRemindMin = 9 * 60

	dateLayout = "2006-01-02"
)

var (
	ErrInvalid              = apperr.NewCoded(apperr.Invalid, "goal_invalid", "dados da meta inválidos")
	ErrAmountInvalid        = apperr.NewCoded(apperr.Invalid, "amount_invalid", "valor inválido")
	ErrNotFound             = apperr.NewCoded(apperr.NotFound, "goal_not_found", "meta não encontrada")
	ErrStepNotFound         = apperr.NewCoded(apperr.NotFound, "step_not_found", "etapa não encontrada")
	ErrContributionNotFound = apperr.NewCoded(apperr.NotFound, "contribution_not_found", "aporte não encontrado")
	ErrForbidden            = apperr.NewCoded(apperr.Forbidden, "goal_forbidden", "você não pode alterar esta meta")
	ErrConflict             = apperr.NewCoded(apperr.Conflict, "goal_conflict", "a meta foi alterada por outra pessoa; recarregue")
	ErrClosed               = apperr.NewCoded(apperr.Conflict, "goal_closed", "a meta está concluída ou arquivada")
	ErrKindMismatch         = apperr.NewCoded(apperr.Invalid, "goal_kind_mismatch", "isso não vale para este tipo de meta")
	ErrLimit                = apperr.NewCoded(apperr.Conflict, "goal_limit", "limite de metas ou etapas atingido")
	ErrContributionNegative = apperr.NewCoded(apperr.Conflict, "contribution_negative", "a retirada é maior que o valor guardado")
	ErrCheckinDate          = apperr.NewCoded(apperr.Invalid, "checkin_date_invalid", "a marcação não pode ser no futuro nem de mais de 31 dias atrás")
	ErrModuleRequired       = apperr.NewCoded(apperr.Conflict, "module_required", "esta meta precisa do módulo Finanças ligado")
	ErrStepHasTask          = apperr.NewCoded(apperr.Conflict, "step_has_task", "a etapa tem uma tarefa: conclua ou reabra pela tarefa")
)

// Person é um membro do casal, com o fuso em que conta os dias.
type Person struct {
	ID       string
	Name     string
	Timezone string
}

// Goal é uma meta com os parâmetros do seu tipo e o progresso já calculado pelo servidor.
// Quem só enxerga (Access = view) recebe título, prazo e progresso: o resto vem vazio.
type Goal struct {
	ID          string     `json:"id"`
	HouseholdID string     `json:"-"`
	CreatedBy   string     `json:"created_by"`
	OwnerID     *string    `json:"owner_id"` // nulo: meta do casal
	Scope       string     `json:"scope"`
	Visibility  string     `json:"visibility"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	Notes       string     `json:"notes,omitempty"`
	Status      string     `json:"status"`
	DueOn       *string    `json:"due_on"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CompletedBy *string    `json:"completed_by,omitempty"`
	Version     int        `json:"version"`
	Access      string     `json:"access"`
	CanEdit     bool       `json:"can_edit"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Financial *Financial `json:"financial,omitempty"`
	Habit     *Habit     `json:"habit,omitempty"`
	Shared    *Shared    `json:"shared,omitempty"`
	Steps     []Step     `json:"steps,omitempty"`

	Progress Progress `json:"progress"`
	Recent   *Recent  `json:"recent,omitempty"` // só na leitura de uma meta

	// SyncPending avisa que a meta foi gravada, mas a tarefa de uma etapa será refeita pelo job.
	SyncPending bool `json:"sync_pending,omitempty"`
}

type Financial struct {
	TargetCents int64   `json:"target_cents"`
	Source      string  `json:"source"`
	CategoryID  *string `json:"category_id,omitempty"`
	BaselineOn  string  `json:"baseline_on"`
}

// Habit.Weekdays é uma máscara: bit 0 = segunda ... bit 6 = domingo; 0 significa qualquer dia.
type Habit struct {
	TargetCount   int    `json:"target_count"`
	Period        string `json:"period"`
	Weekdays      int    `json:"weekdays"`
	RemindMin     int    `json:"remind_min"` // minuto do dia, no fuso do dono
	RemindEnabled bool   `json:"remind_enabled"`
}

type Shared struct {
	TargetCount  int    `json:"target_count"`
	Period       string `json:"period"`
	RequiresBoth bool   `json:"requires_both"`
}

// Step é uma etapa de projeto. Com tarefa (HasTask), Done e AssigneeID vêm da tarefa, nunca de um segundo lugar.
type Step struct {
	ID         string     `json:"id"`
	Position   int        `json:"position"`
	Title      string     `json:"title"`
	AssigneeID *string    `json:"assignee_id"`
	DueOn      *string    `json:"due_on,omitempty"`
	Done       bool       `json:"done"`
	DoneAt     *time.Time `json:"done_at,omitempty"`
	HasTask    bool       `json:"has_task"`
	TaskID     *string    `json:"task_id,omitempty"`
	// TaskStatus: open, done ou declined (tarefa recusada deixa a etapa aberta e sem responsável)
	TaskStatus string `json:"task_status,omitempty"`

	// campos internos
	goalID      string
	wantTask    bool
	taskID      string
	rawDone     *time.Time
	rawAssignee *string
}

// Progress é calculado só no servidor; o cliente nunca soma nada.
type Progress struct {
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	Percent int    `json:"percent"` // inteiro, arredondado para baixo; pode passar de 100 em hábito e meta do casal
	Unit    string `json:"unit"`    // cents, count ou steps

	OnTrack               *bool  `json:"on_track,omitempty"`
	RequiredPerMonthCents *int64 `json:"required_per_month_cents,omitempty"`
	Provisional           bool   `json:"provisional,omitempty"` // budget_savings: o mês corrente ainda pode mudar
	Unavailable           bool   `json:"unavailable,omitempty"` // budget_savings com Finanças desligado
	SuggestComplete       bool   `json:"suggest_complete,omitempty"`
	Period                *Range `json:"period,omitempty"`
	MarkedToday           bool   `json:"marked_today,omitempty"`
	// PendingDays: meta do casal que exige os dois, dias do período em que só uma pessoa marcou ("falta a outra pessoa")
	PendingDays []string `json:"pending_days,omitempty"`
}

// Range é um intervalo de datas, com as duas pontas incluídas.
type Range struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Recent struct {
	Contributions []Contribution `json:"contributions,omitempty"`
	Checkins      []DayMarks     `json:"checkins,omitempty"`
}

type Contribution struct {
	ID            string    `json:"id"`
	AmountCents   int64     `json:"amount_cents"`
	MadeOn        string    `json:"made_on"`
	Note          string    `json:"note,omitempty"`
	CreatedBy     string    `json:"created_by"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
}

// Checkin é uma marcação de uma pessoa num dia.
type Checkin struct {
	UserID string `json:"user_id"`
	Day    string `json:"day"`
}

// DayMarks são as marcações de um dia. By vem em ordem de id, sem significado: nenhuma resposta ordena as pessoas.
type DayMarks struct {
	Day     string   `json:"day"`
	By      []string `json:"by"`
	Counted bool     `json:"counted"` // o dia conta para o progresso
}

type PeriodSummary struct {
	Start  string `json:"start"`
	End    string `json:"end"`
	Count  int    `json:"count"`
	Target int    `json:"target"`
}

type History struct {
	From    string          `json:"from"`
	To      string          `json:"to"`
	Days    []DayMarks      `json:"days"`
	Periods []PeriodSummary `json:"periods"`
}

type ContributionPage struct {
	Items      []Contribution `json:"items"`
	NextBefore string         `json:"next_before,omitempty"`
}

type ListFilter struct {
	Scope  string // "" (tudo o que a pessoa vê), household ou mine
	Kind   string
	Status string // "" lista as não arquivadas
}

// ---- entradas ----

type FinancialInput struct {
	TargetCents int64  `json:"target_cents"`
	Source      string `json:"source"`
	CategoryID  string `json:"category_id"`
	BaselineOn  string `json:"baseline_on"`
}

type HabitInput struct {
	TargetCount   int    `json:"target_count"`
	Period        string `json:"period"`
	Weekdays      int    `json:"weekdays"`
	RemindEnabled bool   `json:"remind_enabled"`
	RemindMin     *int   `json:"remind_min"`
}

type SharedInput struct {
	TargetCount  int    `json:"target_count"`
	Period       string `json:"period"`
	RequiresBoth bool   `json:"requires_both"`
}

type StepInput struct {
	Title      string `json:"title"`
	AssigneeID string `json:"assignee_id"`
	DueOn      string `json:"due_on"`
	Task       bool   `json:"task"` // já cria a tarefa (só em projeto do casal)
}

type CreateInput struct {
	Kind       string          `json:"kind"`
	Scope      string          `json:"scope"`
	Visibility string          `json:"visibility"`
	Title      string          `json:"title"`
	Notes      string          `json:"notes"`
	DueOn      string          `json:"due_on"`
	Financial  *FinancialInput `json:"financial"`
	Habit      *HabitInput     `json:"habit"`
	Shared     *SharedInput    `json:"shared"`
	Steps      []StepInput     `json:"steps"`
}

// UpdateInput substitui título, notas, prazo e visibilidade. Os blocos do tipo mexem só no que pode mudar sem
// reescrever o histórico (alvo, dias, lembrete); o tipo e o período não mudam.
type UpdateInput struct {
	Version    int    `json:"version"`
	Title      string `json:"title"`
	Notes      string `json:"notes"`
	DueOn      string `json:"due_on"`
	Visibility string `json:"visibility"`

	Financial *struct {
		TargetCents int64 `json:"target_cents"`
	} `json:"financial"`
	Habit *struct {
		TargetCount   int  `json:"target_count"`
		Weekdays      int  `json:"weekdays"`
		RemindEnabled bool `json:"remind_enabled"`
		RemindMin     *int `json:"remind_min"`
	} `json:"habit"`
	Shared *struct {
		TargetCount  int  `json:"target_count"`
		RequiresBoth bool `json:"requires_both"`
	} `json:"shared"`
}

type ContributionInput struct {
	AmountCents int64  `json:"amount_cents"`
	MadeOn      string `json:"made_on"`
	Note        string `json:"note"`
}

type StepUpdate struct {
	Title      *string `json:"title"`
	AssigneeID *string `json:"assignee_id"` // "" tira o responsável
	DueOn      *string `json:"due_on"`      // "" tira o prazo
	Position   *int    `json:"position"`
}
