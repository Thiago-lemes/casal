import { api } from '@/shared/api/client'

/** Tudo em centavos inteiros; datas "AAAA-MM-DD"; meses "AAAA-MM". */

export type Category = { id: string; name: string; key?: string; archived: boolean }

/** Cartão de crédito. É pessoal: o parceiro nunca o vê. */
export type Card = {
  id: string
  name: string
  brand?: string
  last4?: string
  closing_day?: number
  due_day?: number
  archived: boolean
}

export type CardInput = { name: string; brand?: string; last4?: string; closing_day?: number; due_day?: number }

/** Pontos-base por pessoa (1% = 100); a soma é 10000. */
export type Split = Record<string, number>

export type Expense = {
  id: string
  created_by: string
  updated_by?: string
  paid_by: string
  scope: 'household' | 'personal'
  visibility: 'household' | 'private' | 'view'
  amount_cents: number
  category_id: string
  spent_on: string
  description: string
  split?: Split
  /** só vem para quem pagou com o cartão */
  card_id?: string
  source: 'manual' | 'shopping' | 'recurring' | 'bill'
  recurring_id?: string
  bill_id?: string
  installment?: { group_id: string; no: number; total: number }
  settlement_id?: string
  settled: boolean
  can_edit: boolean
  version: number
  created_at: string
  updated_at: string
}

export type ExpensePage = { expenses: Expense[]; next?: string }
export type Group = { group_id: string; expenses: Expense[]; settled: number }

export type ExpenseInput = {
  scope: 'household' | 'personal'
  visibility?: 'private' | 'view'
  paid_by?: string
  amount_cents?: number
  category_id: string
  spent_on: string
  description: string
  split?: Split
  /** na edição, vazio tira o cartão; fora da edição, omita para "sem cartão" */
  card_id?: string
  version?: number
  installments?: { count: number; first_on: string; installment_amount_cents?: number }
}

export type CreateExpenseResult = Expense & { split_fallback?: boolean }
export type CreateInstallmentsResult = { expenses: Expense[]; split_fallback?: boolean }

export type GroupInput = { description?: string; category_id?: string; paid_by?: string; split?: Split; card_id?: string }

export type Balance = {
  through: string
  from_user?: string
  to_user?: string
  amount_cents: number
  paid_cents: Record<string, number>
  expenses: Expense[]
}

export type Settlement = {
  id: string
  through_on: string
  from_user: string
  to_user: string
  amount_cents: number
  kind: 'manual' | 'leave'
  created_by: string
  created_at: string
}

export type SettingsView = {
  split_mode: 'equal' | 'fixed' | 'proportional'
  fixed_split?: Split
  effective_split: Split
  proportional: {
    available: boolean
    split?: Split
    consented_by: string[]
    /** quem informou a renda; nunca os valores */
    income_set: Record<string, boolean>
  }
}

export type SettingsInput = { split_mode: SettingsView['split_mode']; fixed_split?: Split }
export type Income = { monthly_income_cents: number | null }

export type Recurring = {
  id: string
  created_by: string
  paid_by: string
  amount_cents: number
  category_id: string
  description: string
  scope: 'household' | 'personal'
  visibility: string
  split?: Split
  day_of_month: number
  start_on: string
  end_on?: string
  active: boolean
  can_edit: boolean
}

export type RecurringInput = {
  scope: 'household' | 'personal'
  visibility?: 'private' | 'view'
  paid_by?: string
  amount_cents: number
  category_id: string
  description: string
  split?: Split
  day_of_month: number
  start_on: string
  end_on?: string
}

export type Bill = {
  id: string
  created_by: string
  title: string
  amount_cents?: number
  due_on: string
  responsible_id: string
  category_id: string
  notify_both: boolean
  repeat: 'none' | 'monthly'
  paid_at?: string
  paid_by?: string
  expense_id?: string
  /** gravada, mas os lembretes serão refeitos em até uma hora */
  sync_pending?: boolean
}

export type BillInput = {
  title: string
  amount_cents: number | null
  due_on: string
  responsible_id: string
  category_id: string
  notify_both: boolean
  repeat: 'none' | 'monthly'
}

export type PayInput = { amount_cents?: number; paid_on?: string; paid_by?: string }
export type PayResult = { bill: Bill; expense: Expense; next_bill?: Bill; split_fallback?: boolean }

export type Budget = { category_id: string; limit_cents: number }

export type Summary = {
  month: string
  scope: 'household' | 'mine'
  total_cents: number
  previous_total_cents: number
  categories: {
    category_id: string
    total_cents: number
    previous_cents: number
    limit_cents?: number
    used_percent?: number
  }[]
  paid_by?: Record<string, number>
  /** totais nos MEUS cartões; o cartão do parceiro não aparece */
  cards: { card_id: string; total_cents: number }[]
  without_card_cents: number
}

export type ExpenseQuery = { month: string; scope: 'household' | 'mine'; before?: string; limit?: number }

const qs = (params: Record<string, string | number | undefined>) => {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== '') q.set(k, String(v))
  const s = q.toString()
  return s ? `?${s}` : ''
}

const json = (method: string, body?: unknown) => ({ method, body })

/** Um objeto com uma função por rota. O servidor é a autoridade em tudo que é conta. */
export const financeApi = {
  categories: async (archived = false) =>
    (await api<Category[] | null>(`/v1/finance/categories${qs({ archived: archived ? 'true' : undefined })}`)) ?? [],
  createCategory: (name: string) => api<Category>('/v1/finance/categories', json('POST', { name })),
  renameCategory: (id: string, name: string) => api<Category>(`/v1/finance/categories/${id}`, json('PUT', { name })),
  archiveCategory: (id: string) => api<Category>(`/v1/finance/categories/${id}/archive`, json('POST')),
  unarchiveCategory: (id: string) => api<Category>(`/v1/finance/categories/${id}/unarchive`, json('POST')),

  cards: async (archived = false) => (await api<Card[] | null>(`/v1/finance/cards${qs({ archived: archived ? 'true' : undefined })}`)) ?? [],
  createCard: (v: CardInput) => api<Card>('/v1/finance/cards', json('POST', v)),
  updateCard: (id: string, v: CardInput) => api<Card>(`/v1/finance/cards/${id}`, json('PUT', v)),
  archiveCard: (id: string) => api<Card>(`/v1/finance/cards/${id}/archive`, json('POST')),
  unarchiveCard: (id: string) => api<Card>(`/v1/finance/cards/${id}/unarchive`, json('POST')),

  expenses: async (q: ExpenseQuery) => {
    const page = await api<ExpensePage>(`/v1/finance/expenses${qs({ month: q.month, scope: q.scope, before: q.before, limit: q.limit ?? 50 })}`)
    return { ...page, expenses: page.expenses ?? [] }
  },
  createExpense: (v: ExpenseInput) => api<CreateExpenseResult | CreateInstallmentsResult>('/v1/finance/expenses', json('POST', v)),
  expense: (id: string) => api<Expense>(`/v1/finance/expenses/${id}`),
  updateExpense: (id: string, v: ExpenseInput) => api<Expense>(`/v1/finance/expenses/${id}`, json('PUT', v)),
  deleteExpense: (id: string) => api<void>(`/v1/finance/expenses/${id}`, json('DELETE')),
  fromShopping: (v: { amount_cents: number; paid_by?: string; description: string; spent_on?: string }) =>
    api<CreateExpenseResult>('/v1/finance/expenses/from-shopping', json('POST', v)),

  group: async (id: string) => {
    const g = await api<Group>(`/v1/finance/expense-groups/${id}`)
    return { ...g, expenses: g.expenses ?? [] }
  },
  updateGroup: (id: string, v: GroupInput) => api<Group>(`/v1/finance/expense-groups/${id}`, json('PUT', v)),
  deleteGroup: (id: string) => api<{ deleted: number; remaining: number }>(`/v1/finance/expense-groups/${id}`, json('DELETE')),

  balance: async (through?: string) => {
    const b = await api<Balance>(`/v1/finance/balance${qs({ through })}`)
    return { ...b, expenses: b.expenses ?? [], paid_cents: b.paid_cents ?? {} }
  },
  settle: (v: { through_on: string; amount_cents: number }) => api<Settlement>('/v1/finance/settlements', json('POST', v)),
  settlements: async () => (await api<Settlement[] | null>('/v1/finance/settlements')) ?? [],

  settings: () => api<SettingsView>('/v1/finance/settings'),
  saveSettings: (v: SettingsInput) => api<SettingsView>('/v1/finance/settings', json('PUT', v)),
  consent: () => api<SettingsView>('/v1/finance/settings/proportional/consent', json('POST')),
  withdrawConsent: () => api<SettingsView>('/v1/finance/settings/proportional/consent', json('DELETE')),
  income: () => api<Income>('/v1/finance/income'),
  saveIncome: (cents: number) => api<Income>('/v1/finance/income', json('PUT', { monthly_income_cents: cents })),
  removeIncome: () => api<void>('/v1/finance/income', json('DELETE')),

  recurring: async () => (await api<Recurring[] | null>('/v1/finance/recurring')) ?? [],
  createRecurring: (v: RecurringInput) => api<Recurring>('/v1/finance/recurring', json('POST', v)),
  updateRecurring: (id: string, v: RecurringInput) => api<Recurring>(`/v1/finance/recurring/${id}`, json('PUT', v)),
  pauseRecurring: (id: string) => api<Recurring>(`/v1/finance/recurring/${id}/pause`, json('POST')),
  resumeRecurring: (id: string) => api<Recurring>(`/v1/finance/recurring/${id}/resume`, json('POST')),
  deleteRecurring: (id: string) => api<void>(`/v1/finance/recurring/${id}`, json('DELETE')),

  bills: async (q: { status?: 'open' | 'paid'; due?: 'today' } = {}) =>
    (await api<Bill[] | null>(`/v1/finance/bills${qs({ status: q.status, due: q.due })}`)) ?? [],
  createBill: (v: BillInput) => api<Bill>('/v1/finance/bills', json('POST', v)),
  updateBill: (id: string, v: BillInput) => api<Bill>(`/v1/finance/bills/${id}`, json('PUT', v)),
  deleteBill: (id: string) => api<void>(`/v1/finance/bills/${id}`, json('DELETE')),
  payBill: (id: string, v: PayInput) => api<PayResult>(`/v1/finance/bills/${id}/pay`, json('POST', v)),

  budgets: async () => (await api<Budget[] | null>('/v1/finance/budgets')) ?? [],
  saveBudget: (v: Budget) => api<Budget>('/v1/finance/budgets', json('PUT', v)),
  removeBudget: (categoryId: string) => api<void>(`/v1/finance/budgets/${categoryId}`, json('DELETE')),
  summary: async (month: string, scope: 'household' | 'mine') => {
    const s = await api<Summary>(`/v1/finance/summary${qs({ month, scope })}`)
    return { ...s, categories: s.categories ?? [], cards: s.cards ?? [], without_card_cents: s.without_card_cents ?? 0 }
  },
}

/** Link direto para o navegador baixar (sem fetch + blob). */
export const exportUrl = (format: 'csv' | 'json' = 'csv') => `/v1/finance/export?format=${format}`

export function isInstallmentsResult(r: CreateExpenseResult | CreateInstallmentsResult): r is CreateInstallmentsResult {
  return 'expenses' in r
}
