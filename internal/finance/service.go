// Package finance é o módulo Finanças: gastos, divisão e acerto de contas, contas a pagar e orçamento.
// Valores sempre em centavos inteiros. Os gastos pessoais e a renda são privados entre os parceiros, e essa trava vive
// só no código (o RLS não separa os dois do mesmo casal): toda leitura passa por visibleTo / filtra por quem pergunta.
package finance

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"casal/internal/identity"
)

// Person é um membro do casal, com o fuso em que recebe os lembretes.
type Person struct {
	ID       string
	Name     string
	Timezone string
}

// People é o que Finanças precisa da feature household (o app liga as duas).
type People interface {
	Members(ctx context.Context, householdID string) ([]Person, error)
}

// Reminders é o que Finanças precisa da feature reminder.
type Reminders interface {
	CreateWithRef(ctx context.Context, householdID, creatorID, recipientID, body string, remindAt time.Time, ref string) error
	CancelByRef(ctx context.Context, householdID, ref string) error
}

// Notifier envia um aviso a uma pessoa. Nunca é chamado dentro de uma transação.
type Notifier interface {
	Notify(ctx context.Context, userID, title, body string) error
}

type Service struct {
	repo      Repository
	people    People
	reminders Reminders
	notifier  Notifier
	log       *slog.Logger
	now       func() time.Time
}

func NewService(repo Repository, people People, reminders Reminders, notifier Notifier, log *slog.Logger) *Service {
	return &Service{repo: repo, people: people, reminders: reminders, notifier: notifier, log: log, now: time.Now}
}

func (s *Service) members(ctx context.Context, hh string) ([]Person, error) {
	ps, err := s.people.Members(ctx, hh)
	if err != nil {
		return nil, err
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].ID < ps[j].ID })
	return ps, nil
}

func ids(ps []Person) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.ID
	}
	return out
}

func isMember(ps []Person, id string) bool {
	for _, p := range ps {
		if p.ID == id {
			return true
		}
	}
	return false
}

func other(ps []Person, me string) string {
	for _, p := range ps {
		if p.ID != me {
			return p.ID
		}
	}
	return ""
}

// notify avisa em segundo plano de erro: o aviso nunca desfaz o que já foi gravado.
func (s *Service) notify(ctx context.Context, userID, title, body string) {
	if userID == "" {
		return
	}
	if err := s.notifier.Notify(ctx, userID, title, body); err != nil {
		s.log.Warn("falha ao avisar", "user", userID, "title", title, "err", err)
	}
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func checkText(s string, max int, what string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		return "", fmt.Errorf("%w: %s tem até %d caracteres", ErrInvalidInput, what, max)
	}
	return s, nil
}

// ---- categorias ----

func (s *Service) ListCategories(ctx context.Context, p identity.Principal, archived bool) ([]Category, error) {
	return s.repo.ListCategories(ctx, p.HouseholdID, archived)
}

func (s *Service) categoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxCategory {
		return "", fmt.Errorf("%w: o nome da categoria tem de 1 a %d caracteres", ErrInvalidInput, maxCategory)
	}
	return name, nil
}

func (s *Service) CreateCategory(ctx context.Context, p identity.Principal, name string) (Category, error) {
	name, err := s.categoryName(name)
	if err != nil {
		return Category{}, err
	}
	return s.repo.CreateCategory(ctx, p.HouseholdID, name)
}

func (s *Service) RenameCategory(ctx context.Context, p identity.Principal, id, name string) (Category, error) {
	name, err := s.categoryName(name)
	if err != nil {
		return Category{}, err
	}
	return s.repo.RenameCategory(ctx, p.HouseholdID, id, name)
}

func (s *Service) ArchiveCategory(ctx context.Context, p identity.Principal, id string, archived bool) (Category, error) {
	return s.repo.SetCategoryArchived(ctx, p.HouseholdID, id, archived)
}

// usableCategory confere que a categoria é do casal e aceita gasto novo.
func (s *Service) usableCategory(ctx context.Context, hh, id string) error {
	c, err := s.repo.GetCategory(ctx, hh, id)
	if err != nil {
		return err
	}
	if c.Archived {
		return ErrCategoryArchived
	}
	return nil
}

// ---- divisão padrão ----

// proportionalSplit calcula a divisão pela renda. Só existe com duas pessoas, as duas rendas informadas e as duas
// concordâncias. Os valores das rendas ficam aqui dentro: só a divisão resultante sai.
func (s *Service) proportionalSplit(ctx context.Context, hh string, st Settings, ps []Person) (map[string]int, bool, error) {
	if len(ps) != 2 || !containsID(st.Consent, ps[0].ID) || !containsID(st.Consent, ps[1].ID) {
		return nil, false, nil
	}
	incomes, err := s.repo.Incomes(ctx, hh, ids(ps))
	if err != nil {
		return nil, false, err
	}
	sp, ok := ProportionalSplit(ids(ps), incomes)
	return sp, ok, nil
}

// defaultSplit é a divisão que vale para um gasto novo do casal quando a pessoa não envia uma. fallback avisa que o
// casal está em proporcional, mas ela não está disponível, e o gasto caiu em igual.
func (s *Service) defaultSplit(ctx context.Context, hh string, ps []Person) (split map[string]int, fallback bool, err error) {
	st, err := s.repo.GetSettings(ctx, hh)
	if err != nil {
		return nil, false, err
	}
	switch st.Mode {
	case ModeFixed:
		if sp, err := NormalizeSplit(st.FixedSplit, ids(ps)); err == nil {
			return sp, false, nil
		}
	case ModeProportional:
		sp, ok, err := s.proportionalSplit(ctx, hh, st, ps)
		if err != nil {
			return nil, false, err
		}
		if ok {
			return sp, false, nil
		}
		return EqualSplit(ids(ps)), true, nil
	}
	return EqualSplit(ids(ps)), false, nil
}

// resolveSplit: a divisão enviada no gasto; senão a do casal.
func (s *Service) resolveSplit(ctx context.Context, hh string, given map[string]int, ps []Person) (map[string]int, bool, error) {
	if given != nil {
		sp, err := NormalizeSplit(given, ids(ps))
		return sp, false, err
	}
	return s.defaultSplit(ctx, hh, ps)
}

type ProportionalView struct {
	Available bool            `json:"available"`
	Split     map[string]int  `json:"split,omitempty"`
	Consented []string        `json:"consented_by"`
	IncomeSet map[string]bool `json:"income_set"`
}

// SettingsView é a divisão padrão do casal. Nunca traz as rendas, só a divisão resultante.
type SettingsView struct {
	Mode           string           `json:"split_mode"`
	FixedSplit     map[string]int   `json:"fixed_split,omitempty"`
	EffectiveSplit map[string]int   `json:"effective_split"`
	Proportional   ProportionalView `json:"proportional"`
}

func (s *Service) settingsView(ctx context.Context, hh string, st Settings, ps []Person) (SettingsView, error) {
	incomes, err := s.repo.Incomes(ctx, hh, ids(ps))
	if err != nil {
		return SettingsView{}, err
	}
	v := SettingsView{Mode: st.Mode, FixedSplit: st.FixedSplit, Proportional: ProportionalView{
		Consented: []string{}, IncomeSet: map[string]bool{},
	}}
	for _, p := range ps {
		v.Proportional.IncomeSet[p.ID] = incomes[p.ID] > 0
		if containsID(st.Consent, p.ID) {
			v.Proportional.Consented = append(v.Proportional.Consented, p.ID)
		}
	}
	if sp, ok, err := s.proportionalSplit(ctx, hh, st, ps); err != nil {
		return SettingsView{}, err
	} else if ok {
		v.Proportional.Available, v.Proportional.Split = true, sp
	}
	v.EffectiveSplit, _, err = s.defaultSplit(ctx, hh, ps)
	return v, err
}

func (s *Service) GetSettings(ctx context.Context, p identity.Principal) (SettingsView, error) {
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return SettingsView{}, err
	}
	st, err := s.repo.GetSettings(ctx, p.HouseholdID)
	if err != nil {
		return SettingsView{}, err
	}
	return s.settingsView(ctx, p.HouseholdID, st, ps)
}

func (s *Service) SaveSettings(ctx context.Context, p identity.Principal, mode string, fixed map[string]int) (SettingsView, error) {
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return SettingsView{}, err
	}
	cur, err := s.repo.GetSettings(ctx, p.HouseholdID)
	if err != nil {
		return SettingsView{}, err
	}
	next := Settings{Mode: mode}
	switch mode {
	case ModeEqual:
	case ModeFixed:
		if next.FixedSplit, err = NormalizeSplit(fixed, ids(ps)); err != nil {
			return SettingsView{}, err
		}
	case ModeProportional:
		if _, ok, err := s.proportionalSplit(ctx, p.HouseholdID, cur, ps); err != nil {
			return SettingsView{}, err
		} else if !ok {
			return SettingsView{}, ErrProportionalOff
		}
	default:
		return SettingsView{}, fmt.Errorf("%w: modo de divisão inválido", ErrInvalidInput)
	}
	if err := s.repo.SaveSettings(ctx, p.HouseholdID, next); err != nil {
		return SettingsView{}, err
	}
	if cur.Mode != mode {
		s.notify(ctx, other(ps, p.UserID), "A divisão padrão mudou", "Confira como os próximos gastos serão divididos.")
	}
	return s.GetSettings(ctx, p)
}

func (s *Service) Consent(ctx context.Context, p identity.Principal) (SettingsView, error) {
	if _, err := s.repo.AddConsent(ctx, p.HouseholdID, p.UserID); err != nil {
		return SettingsView{}, err
	}
	return s.GetSettings(ctx, p)
}

func (s *Service) WithdrawConsent(ctx context.Context, p identity.Principal) (SettingsView, error) {
	_, reset, err := s.repo.RemoveConsent(ctx, p.HouseholdID, p.UserID)
	if err != nil {
		return SettingsView{}, err
	}
	if reset {
		s.notifyDivisionChanged(ctx, p)
	}
	return s.GetSettings(ctx, p)
}

func (s *Service) notifyDivisionChanged(ctx context.Context, p identity.Principal) {
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		s.log.Warn("falha ao avisar da divisão", "err", err)
		return
	}
	// só avisa que mudou: nunca o motivo nem valores (a renda é privada)
	s.notify(ctx, other(ps, p.UserID), "A divisão padrão mudou", "Confira como os próximos gastos serão divididos.")
}

// ---- renda (privada) ----

// GetIncome devolve só a renda de quem pergunta.
func (s *Service) GetIncome(ctx context.Context, p identity.Principal) (*int64, error) {
	c, ok, err := s.repo.GetIncome(ctx, p.HouseholdID, p.UserID)
	if err != nil || !ok {
		return nil, err
	}
	return &c, nil
}

func (s *Service) SetIncome(ctx context.Context, p identity.Principal, cents int64) (int64, error) {
	if !validAmount(cents) {
		return 0, ErrIncomeInvalid
	}
	if err := s.repo.SetIncome(ctx, p.HouseholdID, p.UserID, cents); err != nil {
		return 0, err
	}
	st, err := s.repo.GetSettings(ctx, p.HouseholdID)
	if err != nil {
		return 0, err
	}
	if st.Mode == ModeProportional {
		s.notifyDivisionChanged(ctx, p)
	}
	return cents, nil
}

func (s *Service) DeleteIncome(ctx context.Context, p identity.Principal) error {
	reset, err := s.repo.DeleteIncome(ctx, p.HouseholdID, p.UserID)
	if err != nil {
		return err
	}
	if reset {
		s.notifyDivisionChanged(ctx, p)
	}
	return nil
}

// ---- saldo e acerto ----

type BalanceView struct {
	Through     string           `json:"through"`
	FromUser    string           `json:"from_user,omitempty"` // quem deve
	ToUser      string           `json:"to_user,omitempty"`   // a quem
	AmountCents int64            `json:"amount_cents"`
	PaidCents   map[string]int64 `json:"paid_cents"` // quanto cada um pagou pelo casal
	Expenses    []Expense        `json:"expenses"`
}

func (s *Service) throughDate(p identity.Principal, through string) (string, error) {
	if through == "" {
		return fmtDate(today(s.now(), p.Timezone)), nil
	}
	t, err := parseDate(through)
	if err != nil {
		return "", err
	}
	return fmtDate(t), nil
}

func (s *Service) Balance(ctx context.Context, p identity.Principal, through string) (BalanceView, error) {
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return BalanceView{}, err
	}
	if len(ps) < 2 {
		return BalanceView{}, ErrSingleMember
	}
	if through, err = s.throughDate(p, through); err != nil {
		return BalanceView{}, err
	}
	open, err := s.repo.OpenExpenses(ctx, p.HouseholdID, through)
	if err != nil {
		return BalanceView{}, err
	}
	from, to, owed, paid := ComputeBalance(open)
	for i := range open {
		open[i].CanEdit = true
	}
	return BalanceView{Through: through, FromUser: from, ToUser: to, AmountCents: owed, PaidCents: paid, Expenses: open}, nil
}

// Settle registra o "pagamento entre nós" feito por fora do app. O valor tem de ser exatamente o saldo de agora.
func (s *Service) Settle(ctx context.Context, p identity.Principal, through string, amount int64) (Settlement, error) {
	if amount <= 0 {
		return Settlement{}, ErrAmountInvalid
	}
	bal, err := s.Balance(ctx, p, through)
	if err != nil {
		return Settlement{}, err
	}
	if bal.AmountCents == 0 {
		return Settlement{}, ErrNothingToSettle
	}
	st, err := s.repo.CreateSettlement(ctx, p.HouseholdID, p.UserID, bal.Through, amount)
	if err != nil {
		return Settlement{}, err
	}
	ps, err := s.members(ctx, p.HouseholdID)
	if err == nil {
		s.notify(ctx, other(ps, p.UserID), "Acerto de contas registrado", "O saldo entre vocês foi zerado.")
	}
	return st, nil
}

func (s *Service) ListSettlements(ctx context.Context, p identity.Principal) ([]Settlement, error) {
	return s.repo.ListSettlements(ctx, p.HouseholdID)
}
