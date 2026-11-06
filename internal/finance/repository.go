package finance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"casal/internal/db"
)

// Repository é tudo o que o service precisa do banco. Todo método leva o casal e filtra por ele; a privacidade
// entre os parceiros (gasto pessoal, renda) vive só no código e por isso quem lê recebe também quem está perguntando.
type Repository interface {
	// cartões (pessoais: toda consulta filtra por dono)
	ListCards(ctx context.Context, hh, owner string, archived bool) ([]Card, error)
	GetCard(ctx context.Context, hh, owner, id string) (Card, error)
	CreateCard(ctx context.Context, c Card) (Card, error)
	UpdateCard(ctx context.Context, c Card) (Card, error)
	SetCardArchived(ctx context.Context, hh, owner, id string, archived bool) (Card, error)
	// CardTotals soma o mês nos cartões de quem pergunta e nos gastos sem cartão (cartão do parceiro não entra em nenhum).
	CardTotals(ctx context.Context, hh, viewer, scope, month string) (byCard map[string]int64, without int64, err error)

	// categorias
	ListCategories(ctx context.Context, hh string, archived bool) ([]Category, error)
	GetCategory(ctx context.Context, hh, id string) (Category, error)
	CategoryByKey(ctx context.Context, hh, key string) (Category, error)
	CreateCategory(ctx context.Context, hh, name string) (Category, error)
	RenameCategory(ctx context.Context, hh, id, name string) (Category, error)
	SetCategoryArchived(ctx context.Context, hh, id string, archived bool) (Category, error)

	// divisão padrão e renda
	GetSettings(ctx context.Context, hh string) (Settings, error)
	SaveSettings(ctx context.Context, hh string, s Settings) error
	AddConsent(ctx context.Context, hh, userID string) (Settings, error)
	// RemoveConsent tira a concordância; se o casal estava em proporcional, volta para igual (reset = true).
	RemoveConsent(ctx context.Context, hh, userID string) (s Settings, reset bool, err error)
	GetIncome(ctx context.Context, hh, userID string) (cents int64, ok bool, err error)
	// Incomes devolve as rendas das pessoas pedidas. Só o cálculo da divisão usa isto: o valor não sai do service.
	Incomes(ctx context.Context, hh string, userIDs []string) (map[string]int64, error)
	SetIncome(ctx context.Context, hh, userID string, cents int64) error
	// DeleteIncome apaga a renda e a concordância da pessoa; em proporcional, o casal volta para igual (reset = true).
	DeleteIncome(ctx context.Context, hh, userID string) (reset bool, err error)

	// gastos
	InsertExpenses(ctx context.Context, hh string, es []Expense) ([]Expense, error)
	GetExpense(ctx context.Context, hh, id, viewer string) (Expense, error)
	ListExpenses(ctx context.Context, hh, viewer string, f ExpenseFilter) ([]Expense, error)
	// UpdateExpense só grava se a versão e o estado (aberto) ainda forem os lidos; sem linha, devolve sql.ErrNoRows.
	UpdateExpense(ctx context.Context, hh string, e Expense, version int, viewer string) (Expense, error)
	DeleteExpense(ctx context.Context, hh, id string) (bool, error)
	GetGroup(ctx context.Context, hh, groupID, viewer string) ([]Expense, error)
	UpdateGroup(ctx context.Context, hh, groupID, viewer string, p GroupPatch) (int64, error)
	DeleteGroup(ctx context.Context, hh, groupID, viewer string) (deleted, remaining int64, err error)
	VisibleExpenses(ctx context.Context, hh, viewer string, limit int) ([]Expense, error)

	// acerto
	OpenExpenses(ctx context.Context, hh, through string) ([]Expense, error)
	CreateSettlement(ctx context.Context, hh, createdBy, through string, amount int64) (Settlement, error)
	ListSettlements(ctx context.Context, hh string) ([]Settlement, error)

	// gastos fixos
	InsertRecurring(ctx context.Context, r Recurring) (Recurring, error)
	ListRecurring(ctx context.Context, hh, viewer string) ([]Recurring, error)
	GetRecurring(ctx context.Context, hh, id, viewer string) (Recurring, error)
	UpdateRecurring(ctx context.Context, r Recurring, viewer string) (Recurring, error)
	DeleteRecurring(ctx context.Context, hh, id, viewer string) (bool, error)
	SetRecurringActive(ctx context.Context, hh, id, viewer string, active bool) (Recurring, error)
	DueRecurring(ctx context.Context) ([]DueRecurring, error)
	// GenerateRecurring grava o gasto do dia, se ainda não foi gerado (idempotente).
	GenerateRecurring(ctx context.Context, r Recurring, spentOn string) (Expense, bool, error)

	// contas a pagar
	InsertBill(ctx context.Context, b Bill) (Bill, error)
	GetBill(ctx context.Context, hh, id string) (Bill, error)
	ListBills(ctx context.Context, hh string, f BillFilter) ([]Bill, error)
	UpdateBill(ctx context.Context, b Bill) (Bill, error)
	DeleteBill(ctx context.Context, hh, id string) (Bill, error)
	PayBill(ctx context.Context, hh, id string, e Expense) (paid Bill, exp Expense, next *Bill, err error)
	SetBillRefs(ctx context.Context, hh, id string, refs []string) error
	// OpenBillsForSync lista as contas abertas dos casais com o módulo ligado (todos os casais).
	OpenBillsForSync(ctx context.Context, limit int) ([]Bill, error)
	BillsWithRefs(ctx context.Context, hh string) ([]Bill, error)

	// orçamento
	ListBudgets(ctx context.Context, hh string) ([]Budget, error)
	SaveBudget(ctx context.Context, hh string, b Budget) error
	DeleteBudget(ctx context.Context, hh, categoryID string) (bool, error)
	BudgetLimit(ctx context.Context, hh, categoryID string) (int64, bool, error)
	MonthUsage(ctx context.Context, hh, categoryID, month string) (int64, error)
	// SpentByMonth é o gasto do casal na categoria por mês (AAAA-MM), de from a to; o gasto pessoal não conta.
	SpentByMonth(ctx context.Context, hh, categoryID string, from, to time.Time) (map[string]int64, error)
	// TryAlert grava o aviso do nível naquele mês; só o primeiro a gravar recebe true.
	TryAlert(ctx context.Context, hh, categoryID, month string, level int) (bool, error)

	// resumo
	CategoryTotals(ctx context.Context, hh, viewer, scope, month string) (map[string]int64, error)
	PaidTotals(ctx context.Context, hh, month string) (map[string]int64, error)
}

// GroupPatch muda só o que vier preenchido, nas parcelas ainda abertas de uma compra parcelada.
type GroupPatch struct {
	Description *string
	CategoryID  *string
	PaidBy      *string
	Split       map[string]int
	UpdatedBy   string
	// CardSet diz que o cartão deve mudar para CardID (vazio = tirar o cartão).
	CardSet bool
	CardID  string
}

// DueRecurring é um gasto fixo ativo com o fuso de quem paga, para o job decidir o dia.
type DueRecurring struct {
	Recurring
	Timezone      string
	LastGenerated string // AAAA-MM-DD, vazio se nunca gerou
}

type BillFilter struct {
	Status    string // open (padrão) ou paid
	DueBefore string // AAAA-MM-DD, inclusive
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

type scanner interface{ Scan(dest ...any) error }

func (p *PostgresRepo) inHousehold(ctx context.Context, hh string, fn func(tx *sql.Tx) error) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: hh}, fn)
}

// nullable devolve nil para texto vazio, para o banco gravar NULL.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func jsonArg(m map[string]int) any {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func parseSplit(raw []byte) map[string]int {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]int
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// uuidArray monta o literal de array do Postgres a partir de ids (uuid), que não precisam de aspas.
func uuidArray(ids []string) string { return "{" + strings.Join(ids, ",") + "}" }

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// ---- categorias ----

func scanCategory(s scanner) (Category, error) {
	var c Category
	var key sql.NullString
	var archived sql.NullTime
	if err := s.Scan(&c.ID, &c.Name, &key, &archived); err != nil {
		return Category{}, err
	}
	c.Key, c.Archived = key.String, archived.Valid
	return c, nil
}

const catCols = `c.id, c.name, c.key, c.archived_at`

// SeedCategories cria as categorias padrão. Roda como gancho do módulo, na transação que o liga; religar não
// recria nem renomeia nada (o que já existe, pela chave ou pelo nome, é ignorado).
func SeedCategories(ctx context.Context, tx *sql.Tx, hh string) error {
	for _, c := range defaultCategories {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_categories (household_id, name, key) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, hh, c.Name, c.Key); err != nil {
			return err
		}
	}
	return nil
}

func (p *PostgresRepo) ListCategories(ctx context.Context, hh string, archived bool) ([]Category, error) {
	out := []Category{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+catCols+` FROM finance_categories c
			WHERE c.household_id = $1 AND ($2 OR c.archived_at IS NULL)
			ORDER BY c.archived_at IS NOT NULL, c.created_at, c.name_key`, hh, archived)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scanCategory(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) oneCategory(ctx context.Context, hh, query string, args ...any) (Category, error) {
	var out Category
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = scanCategory(tx.QueryRowContext(ctx, query, args...))
		return err
	})
	if isNoRows(err) {
		return Category{}, ErrCategoryNotFound
	}
	if db.IsUniqueViolation(err) {
		return Category{}, ErrCategoryDuplicate
	}
	return out, err
}

func (p *PostgresRepo) GetCategory(ctx context.Context, hh, id string) (Category, error) {
	return p.oneCategory(ctx, hh,
		`SELECT `+catCols+` FROM finance_categories c WHERE c.id = $1 AND c.household_id = $2`, id, hh)
}

func (p *PostgresRepo) CategoryByKey(ctx context.Context, hh, key string) (Category, error) {
	return p.oneCategory(ctx, hh,
		`SELECT `+catCols+` FROM finance_categories c WHERE c.household_id = $1 AND c.key = $2`, hh, key)
}

func (p *PostgresRepo) CreateCategory(ctx context.Context, hh, name string) (Category, error) {
	return p.oneCategory(ctx, hh, `
		WITH ins AS (INSERT INTO finance_categories (household_id, name) VALUES ($1, $2) RETURNING *)
		SELECT `+catCols+` FROM ins c`, hh, name)
}

func (p *PostgresRepo) RenameCategory(ctx context.Context, hh, id, name string) (Category, error) {
	return p.oneCategory(ctx, hh, `
		WITH upd AS (UPDATE finance_categories SET name = $3 WHERE id = $1 AND household_id = $2 RETURNING *)
		SELECT `+catCols+` FROM upd c`, id, hh, name)
}

func (p *PostgresRepo) SetCategoryArchived(ctx context.Context, hh, id string, archived bool) (Category, error) {
	return p.oneCategory(ctx, hh, `
		WITH upd AS (
			UPDATE finance_categories SET archived_at = CASE WHEN $3 THEN coalesce(archived_at, now()) ELSE NULL END
			WHERE id = $1 AND household_id = $2 RETURNING *)
		SELECT `+catCols+` FROM upd c`, id, hh, archived)
}

// ---- divisão padrão e renda ----

func readSettings(ctx context.Context, tx *sql.Tx, hh string, lock bool) (Settings, error) {
	s := Settings{Mode: ModeEqual}
	var fixed, consent []byte
	q := `SELECT split_mode, fixed_split, proportional_consent FROM finance_settings WHERE household_id = $1`
	if lock {
		q += ` FOR UPDATE`
	}
	err := tx.QueryRowContext(ctx, q, hh).Scan(&s.Mode, &fixed, &consent)
	if isNoRows(err) {
		return s, nil
	}
	if err != nil {
		return Settings{}, err
	}
	s.FixedSplit = parseSplit(fixed)
	_ = json.Unmarshal(consent, &s.Consent)
	return s, nil
}

func writeSettings(ctx context.Context, tx *sql.Tx, hh string, s Settings) error {
	consent, _ := json.Marshal(append([]string{}, s.Consent...))
	_, err := tx.ExecContext(ctx, `
		INSERT INTO finance_settings (household_id, split_mode, fixed_split, proportional_consent, updated_at)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, now())
		ON CONFLICT (household_id) DO UPDATE
		SET split_mode = EXCLUDED.split_mode, fixed_split = EXCLUDED.fixed_split,
		    proportional_consent = EXCLUDED.proportional_consent, updated_at = now()`,
		hh, s.Mode, jsonArg(s.FixedSplit), string(consent))
	return err
}

func (p *PostgresRepo) GetSettings(ctx context.Context, hh string) (Settings, error) {
	var out Settings
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = readSettings(ctx, tx, hh, false)
		return err
	})
	return out, err
}

// SaveSettings grava o modo e a divisão combinada, mantendo quem já concordou. Duas gravações juntas se enfileiram.
func (p *PostgresRepo) SaveSettings(ctx context.Context, hh string, s Settings) error {
	return p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_settings (household_id) VALUES ($1) ON CONFLICT DO NOTHING`, hh); err != nil {
			return err
		}
		cur, err := readSettings(ctx, tx, hh, true)
		if err != nil {
			return err
		}
		s.Consent = cur.Consent
		return writeSettings(ctx, tx, hh, s)
	})
}

func (p *PostgresRepo) mutateConsent(ctx context.Context, hh string, fn func(*Settings) bool) (Settings, bool, error) {
	var out Settings
	var changed bool
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_settings (household_id) VALUES ($1) ON CONFLICT DO NOTHING`, hh); err != nil {
			return err
		}
		cur, err := readSettings(ctx, tx, hh, true)
		if err != nil {
			return err
		}
		changed = fn(&cur)
		out = cur
		return writeSettings(ctx, tx, hh, cur)
	})
	return out, changed, err
}

func removeID(ids []string, id string) []string {
	out := make([]string, 0, len(ids))
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func (p *PostgresRepo) AddConsent(ctx context.Context, hh, userID string) (Settings, error) {
	s, _, err := p.mutateConsent(ctx, hh, func(s *Settings) bool {
		if containsID(s.Consent, userID) {
			return false
		}
		s.Consent = append(s.Consent, userID)
		return true
	})
	return s, err
}

func (p *PostgresRepo) RemoveConsent(ctx context.Context, hh, userID string) (Settings, bool, error) {
	return p.mutateConsent(ctx, hh, func(s *Settings) bool {
		s.Consent = removeID(s.Consent, userID)
		if s.Mode == ModeProportional {
			s.Mode = ModeEqual
			return true
		}
		return false
	})
}

func (p *PostgresRepo) GetIncome(ctx context.Context, hh, userID string) (int64, bool, error) {
	var cents int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT monthly_income_cents FROM finance_incomes WHERE user_id = $1 AND household_id = $2`,
			userID, hh).Scan(&cents)
	})
	if isNoRows(err) {
		return 0, false, nil
	}
	return cents, err == nil, err
}

func (p *PostgresRepo) Incomes(ctx context.Context, hh string, userIDs []string) (map[string]int64, error) {
	out := map[string]int64{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT user_id, monthly_income_cents FROM finance_incomes
			WHERE household_id = $1 AND user_id = ANY($2::uuid[])`, hh, uuidArray(userIDs))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var c int64
			if err := rows.Scan(&id, &c); err != nil {
				return err
			}
			out[id] = c
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) SetIncome(ctx context.Context, hh, userID string, cents int64) error {
	return p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO finance_incomes (user_id, household_id, monthly_income_cents) VALUES ($1, $2, $3)
			ON CONFLICT (user_id) DO UPDATE SET household_id = EXCLUDED.household_id,
			    monthly_income_cents = EXCLUDED.monthly_income_cents, updated_at = now()`, userID, hh, cents)
		return err
	})
}

func (p *PostgresRepo) DeleteIncome(ctx context.Context, hh, userID string) (bool, error) {
	var reset bool
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM finance_incomes WHERE user_id = $1 AND household_id = $2`, userID, hh); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_settings (household_id) VALUES ($1) ON CONFLICT DO NOTHING`, hh); err != nil {
			return err
		}
		cur, err := readSettings(ctx, tx, hh, true)
		if err != nil {
			return err
		}
		cur.Consent = removeID(cur.Consent, userID)
		if cur.Mode == ModeProportional {
			cur.Mode, reset = ModeEqual, true
		}
		return writeSettings(ctx, tx, hh, cur)
	})
	return reset, err
}
