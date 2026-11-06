package finance

import (
	"time"

	"casal/internal/apperr"
)

var (
	ErrInvalidInput        = apperr.New(apperr.Invalid, "dados inválidos")
	ErrAmountInvalid       = apperr.NewCoded(apperr.Invalid, "amount_invalid", "o valor precisa ser maior que zero e até R$ 10.000.000,00")
	ErrExpenseNotFound     = apperr.NewCoded(apperr.NotFound, "expense_not_found", "gasto não encontrado")
	ErrCategoryNotFound    = apperr.NewCoded(apperr.NotFound, "category_not_found", "categoria não encontrada")
	ErrBillNotFound        = apperr.NewCoded(apperr.NotFound, "bill_not_found", "conta não encontrada")
	ErrRecurringNotFound   = apperr.NewCoded(apperr.NotFound, "recurring_not_found", "gasto fixo não encontrado")
	ErrExpenseForbidden    = apperr.NewCoded(apperr.Forbidden, "expense_forbidden", "você pode ver este gasto, mas não alterá-lo")
	ErrExpenseConflict     = apperr.NewCoded(apperr.Conflict, "expense_conflict", "o gasto foi alterado por outra pessoa; recarregue")
	ErrExpenseSettled      = apperr.NewCoded(apperr.Conflict, "expense_settled", "esse gasto já entrou num acerto e não pode mudar")
	ErrCategoryArchived    = apperr.NewCoded(apperr.Invalid, "category_archived", "essa categoria está arquivada")
	ErrCategoryDuplicate   = apperr.NewCoded(apperr.Conflict, "category_duplicate", "já existe uma categoria com esse nome")
	ErrSplitInvalid        = apperr.NewCoded(apperr.Invalid, "split_invalid", "divisão inválida")
	ErrInstallmentsInvalid = apperr.NewCoded(apperr.Invalid, "installments_invalid", "parcelamento inválido")
	ErrIncomeInvalid       = apperr.NewCoded(apperr.Invalid, "income_invalid", "a renda precisa ser maior que zero e até R$ 10.000.000,00")
	ErrProportionalOff     = apperr.NewCoded(apperr.Invalid, "proportional_unavailable", "a divisão proporcional precisa da renda e da concordância dos dois")
	ErrSettlementStale     = apperr.NewCoded(apperr.Conflict, "settlement_stale", "o saldo mudou; confira o valor de novo")
	ErrNothingToSettle     = apperr.NewCoded(apperr.Invalid, "nothing_to_settle", "não há saldo para acertar")
	ErrSingleMember        = apperr.NewCoded(apperr.Invalid, "single_member", "o acerto precisa de duas pessoas no casal")
	ErrBillAlreadyPaid     = apperr.NewCoded(apperr.Conflict, "bill_already_paid", "essa conta já foi paga")
	ErrCardNotFound        = apperr.NewCoded(apperr.NotFound, "card_not_found", "cartão não encontrado")
	ErrCardArchived        = apperr.NewCoded(apperr.Invalid, "card_archived", "esse cartão está arquivado")
	ErrCardDuplicate       = apperr.NewCoded(apperr.Conflict, "card_duplicate", "você já tem um cartão com esse nome")
	ErrCardInvalid         = apperr.NewCoded(apperr.Invalid, "card_invalid", "o cartão só vale em gasto pago por você")
)

const (
	ScopeHousehold = "household"
	ScopePersonal  = "personal"

	VisHousehold = "household"
	VisPrivate   = "private"
	VisView      = "view"

	SourceManual    = "manual"
	SourceShopping  = "shopping"
	SourceRecurring = "recurring"
	SourceBill      = "bill"

	ModeEqual        = "equal"
	ModeFixed        = "fixed"
	ModeProportional = "proportional"

	KeyGroceries = "groceries"
	KeyBills     = "bills"
	KeyOther     = "other"

	maxDescription = 140
	maxCategory    = 40
	maxTitle       = 120
	maxCardName    = 40
	maxCardBrand   = 20
	pageDefault    = 50
	pageMax        = 200
)

// defaultCategories são criadas na primeira vez que o módulo é ligado; a tela traduz pelo key.
var defaultCategories = []struct{ Key, Name string }{
	{"housing", "Moradia"}, {"groceries", "Mercado"}, {"transport", "Transporte"}, {"leisure", "Lazer"},
	{"health", "Saúde"}, {"bills", "Contas fixas"}, {"other", "Outras"},
}

type Category struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Key      string `json:"key,omitempty"`
	Archived bool   `json:"archived"`
}

// Card é um cartão de crédito pessoal. O parceiro nunca o lê; o gasto do casal pago com ele aparece ao parceiro
// como qualquer outro, sem cartão.
type Card struct {
	ID          string `json:"id"`
	HouseholdID string `json:"-"`
	OwnerID     string `json:"-"`
	Name        string `json:"name"`
	Brand       string `json:"brand,omitempty"`
	Last4       string `json:"last4,omitempty"`
	ClosingDay  *int   `json:"closing_day,omitempty"`
	DueDay      *int   `json:"due_day,omitempty"`
	Archived    bool   `json:"archived"`
}

type Installment struct {
	GroupID string `json:"group_id"`
	No      int    `json:"no"`
	Total   int    `json:"total"`
}

type Expense struct {
	ID          string         `json:"id"`
	HouseholdID string         `json:"-"`
	CreatedBy   string         `json:"created_by"`
	UpdatedBy   string         `json:"updated_by,omitempty"`
	PaidBy      string         `json:"paid_by"`
	Scope       string         `json:"scope"`
	Visibility  string         `json:"visibility"`
	AmountCents int64          `json:"amount_cents"`
	CategoryID  string         `json:"category_id"`
	SpentOn     string         `json:"spent_on"`
	Description string         `json:"description"`
	Split       map[string]int `json:"split,omitempty"`
	Source      string         `json:"source"`
	RecurringID string         `json:"recurring_id,omitempty"`
	BillID      string         `json:"bill_id,omitempty"`
	Installment *Installment   `json:"installment,omitempty"`
	// CardID é o cartão do gasto (vazio = sem cartão). O cartão é pessoal: o campo guarda o valor real, mas só sai
	// no JSON para o dono (MyCardID), nunca para o parceiro.
	CardID       string `json:"-"`
	MyCardID     string `json:"card_id,omitempty"`
	SettlementID string `json:"settlement_id,omitempty"`
	Settled      bool   `json:"settled"`
	// CanEdit diz se quem pediu pode alterar o gasto (o gasto pessoal do parceiro em "view" é só leitura)
	CanEdit   bool      `json:"can_edit"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Settings é a divisão padrão do casal.
type Settings struct {
	Mode       string         `json:"split_mode"`
	FixedSplit map[string]int `json:"fixed_split,omitempty"`
	Consent    []string       `json:"-"` // quem concordou com a divisão proporcional
}

type Settlement struct {
	ID          string    `json:"id"`
	ThroughOn   string    `json:"through_on"`
	FromUser    string    `json:"from_user"`
	ToUser      string    `json:"to_user"`
	AmountCents int64     `json:"amount_cents"`
	Kind        string    `json:"kind"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type Recurring struct {
	ID          string         `json:"id"`
	HouseholdID string         `json:"-"`
	CreatedBy   string         `json:"created_by"`
	PaidBy      string         `json:"paid_by"`
	AmountCents int64          `json:"amount_cents"`
	CategoryID  string         `json:"category_id"`
	Description string         `json:"description"`
	Scope       string         `json:"scope"`
	Visibility  string         `json:"visibility"`
	Split       map[string]int `json:"split,omitempty"`
	DayOfMonth  int            `json:"day_of_month"`
	StartOn     string         `json:"start_on"`
	EndOn       string         `json:"end_on,omitempty"`
	Active      bool           `json:"active"`
	CanEdit     bool           `json:"can_edit"`
}

type Bill struct {
	ID            string     `json:"id"`
	HouseholdID   string     `json:"-"`
	CreatedBy     string     `json:"created_by"`
	Title         string     `json:"title"`
	AmountCents   *int64     `json:"amount_cents,omitempty"`
	DueOn         string     `json:"due_on"`
	DueDay        int        `json:"-"`
	ResponsibleID string     `json:"responsible_id"`
	CategoryID    string     `json:"category_id"`
	NotifyBoth    bool       `json:"notify_both"`
	Repeat        string     `json:"repeat"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	PaidBy        string     `json:"paid_by,omitempty"`
	ExpenseID     string     `json:"expense_id,omitempty"`
	ReminderRefs  []string   `json:"-"`
	// SyncPending avisa que a conta foi gravada, mas os lembretes ainda serão refeitos pelo job
	SyncPending bool `json:"sync_pending,omitempty"`
}

type Budget struct {
	CategoryID string `json:"category_id"`
	LimitCents int64  `json:"limit_cents"`
}

// ExpenseFilter são os filtros da lista de gastos.
type ExpenseFilter struct {
	Month      string // AAAA-MM
	Scope      string // household (padrão) ou mine
	CategoryID string
	PaidBy     string
	CardID     string // só os gastos que EU paguei com esse cartão
	Before     string // cursor "AAAA-MM-DD,<id>"
	Limit      int
}
