import { useEffect } from 'react'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { isModuleDisabled, modulesKey } from '@/shared/modules/hooks'
import { financeApi } from './api'
import type { BillInput, ExpenseInput, GroupInput, PayInput, RecurringInput, SettingsInput } from './api'

export const financeKey = ['finance'] as const

export const keys = {
  categories: (archived: boolean) => ['finance', 'categories', archived] as const,
  expenses: (month: string, scope: string) => ['finance', 'expenses', { month, scope }] as const,
  group: (id: string) => ['finance', 'group', id] as const,
  balance: (through: string) => ['finance', 'balance', through] as const,
  settlements: ['finance', 'settlements'] as const,
  settings: ['finance', 'settings'] as const,
  income: ['finance', 'income'] as const,
  recurring: ['finance', 'recurring'] as const,
  bills: (status: string) => ['finance', 'bills', status] as const,
  budgets: ['finance', 'budgets'] as const,
  summary: (month: string, scope: string) => ['finance', 'summary', month, scope] as const,
}

/** Um gasto novo muda a lista, o saldo, o resumo, o orçamento e a compra parcelada. */
function refreshMoney(qc: QueryClient) {
  return Promise.all(
    ['expenses', 'balance', 'summary', 'group'].map((k) => qc.invalidateQueries({ queryKey: ['finance', k] })),
  )
}

/** As categorias; as arquivadas só vêm quando o gerenciador pede. */
export function useCategories(archived = false) {
  return useQuery({ queryKey: keys.categories(archived), queryFn: () => financeApi.categories(archived), staleTime: 60_000 })
}

export function useCategoryActions() {
  const qc = useQueryClient()
  const refresh = () => qc.invalidateQueries({ queryKey: ['finance', 'categories'] })
  return {
    create: useMutation({ mutationFn: financeApi.createCategory, onSuccess: refresh }),
    rename: useMutation({ mutationFn: (v: { id: string; name: string }) => financeApi.renameCategory(v.id, v.name), onSuccess: refresh }),
    archive: useMutation({ mutationFn: financeApi.archiveCategory, onSuccess: refresh }),
    unarchive: useMutation({ mutationFn: financeApi.unarchiveCategory, onSuccess: refresh }),
  }
}

/** Gastos do mês, com "Ver mais" pelo cursor `next`. */
export function useExpenses(month: string, scope: 'household' | 'mine') {
  return useInfiniteQuery({
    queryKey: keys.expenses(month, scope),
    queryFn: ({ pageParam }) => financeApi.expenses({ month, scope, before: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next || undefined,
  })
}

export function useGroup(id: string | undefined) {
  return useQuery({ queryKey: keys.group(id ?? ''), queryFn: () => financeApi.group(id!), enabled: Boolean(id) })
}

export function useExpenseActions() {
  const qc = useQueryClient()
  const done = () => refreshMoney(qc)
  return {
    create: useMutation({ mutationFn: (v: ExpenseInput) => financeApi.createExpense(v), onSuccess: done }),
    // editar com `version` não é otimista: espera o servidor
    update: useMutation({ mutationFn: (v: { id: string; body: ExpenseInput }) => financeApi.updateExpense(v.id, v.body), onSuccess: done }),
    remove: useMutation({ mutationFn: financeApi.deleteExpense, onSuccess: done }),
    updateGroup: useMutation({ mutationFn: (v: { id: string; body: GroupInput }) => financeApi.updateGroup(v.id, v.body), onSuccess: done }),
    removeGroup: useMutation({ mutationFn: financeApi.deleteGroup, onSuccess: done }),
    fromShopping: useMutation({ mutationFn: financeApi.fromShopping, onSuccess: done }),
    reload: done,
  }
}

export function useBalance(through: string, enabled = true) {
  return useQuery({ queryKey: keys.balance(through), queryFn: () => financeApi.balance(through), enabled })
}

export function useSettlements() {
  return useQuery({ queryKey: keys.settlements, queryFn: financeApi.settlements })
}

export function useSettle() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: financeApi.settle,
    // o saldo vai a zero e os gastos aparecem como acertados
    onSuccess: () => qc.invalidateQueries({ queryKey: financeKey }),
  })
}

export function useSettings() {
  return useQuery({ queryKey: keys.settings, queryFn: financeApi.settings })
}

export function useSettingsActions() {
  const qc = useQueryClient()
  const set = (data: unknown) => {
    qc.setQueryData(keys.settings, data)
  }
  return {
    save: useMutation({ mutationFn: (v: SettingsInput) => financeApi.saveSettings(v), onSuccess: set }),
    consent: useMutation({ mutationFn: financeApi.consent, onSuccess: set }),
    withdraw: useMutation({ mutationFn: financeApi.withdrawConsent, onSuccess: set }),
  }
}

/** A renda é só da própria pessoa; não existe consulta à renda do parceiro. */
export function useIncome() {
  return useQuery({ queryKey: keys.income, queryFn: financeApi.income })
}

export function useIncomeActions() {
  const qc = useQueryClient()
  // mudar a renda pode devolver o casal à divisão igual
  const refresh = () => Promise.all([qc.invalidateQueries({ queryKey: keys.income }), qc.invalidateQueries({ queryKey: keys.settings })])
  return {
    save: useMutation({ mutationFn: financeApi.saveIncome, onSuccess: refresh }),
    remove: useMutation({ mutationFn: financeApi.removeIncome, onSuccess: refresh }),
  }
}

export function useRecurring() {
  return useQuery({ queryKey: keys.recurring, queryFn: financeApi.recurring })
}

export function useRecurringActions() {
  const qc = useQueryClient()
  // se o dia do mês já passou, o gasto aparece na hora na lista
  const refresh = () => Promise.all([qc.invalidateQueries({ queryKey: keys.recurring }), refreshMoney(qc)])
  return {
    create: useMutation({ mutationFn: (v: RecurringInput) => financeApi.createRecurring(v), onSuccess: refresh }),
    update: useMutation({ mutationFn: (v: { id: string; body: RecurringInput }) => financeApi.updateRecurring(v.id, v.body), onSuccess: refresh }),
    pause: useMutation({ mutationFn: financeApi.pauseRecurring, onSuccess: refresh }),
    resume: useMutation({ mutationFn: financeApi.resumeRecurring, onSuccess: refresh }),
    remove: useMutation({ mutationFn: financeApi.deleteRecurring, onSuccess: refresh }),
  }
}

export function useBills(status: 'open' | 'paid', enabled = true) {
  return useQuery({ queryKey: keys.bills(status), queryFn: () => financeApi.bills({ status }), enabled })
}

/** Abertas que vencem hoje e as vencidas; serve ao bloco "Contas de hoje". */
export function useBillsDueToday(enabled = true) {
  return useQuery({ queryKey: keys.bills('due-today'), queryFn: () => financeApi.bills({ due: 'today' }), enabled })
}

export function useBillActions() {
  const qc = useQueryClient()
  const bills = () => qc.invalidateQueries({ queryKey: ['finance', 'bills'] })
  return {
    create: useMutation({ mutationFn: (v: BillInput) => financeApi.createBill(v), onSuccess: bills }),
    update: useMutation({ mutationFn: (v: { id: string; body: BillInput }) => financeApi.updateBill(v.id, v.body), onSuccess: bills }),
    remove: useMutation({ mutationFn: financeApi.deleteBill, onSuccess: bills }),
    pay: useMutation({
      mutationFn: (v: { id: string; body: PayInput }) => financeApi.payBill(v.id, v.body),
      onSuccess: () => Promise.all([bills(), refreshMoney(qc)]),
    }),
    reload: bills,
  }
}

export function useBudgets() {
  return useQuery({ queryKey: keys.budgets, queryFn: financeApi.budgets })
}

export function useBudgetActions() {
  const qc = useQueryClient()
  const refresh = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: keys.budgets }),
      qc.invalidateQueries({ queryKey: ['finance', 'summary'] }),
    ])
  return {
    save: useMutation({ mutationFn: financeApi.saveBudget, onSuccess: refresh }),
    remove: useMutation({ mutationFn: financeApi.removeBudget, onSuccess: refresh }),
  }
}

export function useSummary(month: string, scope: 'household' | 'mine') {
  return useQuery({ queryKey: keys.summary(month, scope), queryFn: () => financeApi.summary(month, scope) })
}

/** `403 module_disabled` não é falha: o módulo foi desligado. Atualiza o estado dos módulos e a tela mostra o convite. */
export function useModuleGuard(error: unknown) {
  const qc = useQueryClient()
  useEffect(() => {
    if (isModuleDisabled(error)) void qc.invalidateQueries({ queryKey: modulesKey })
  }, [error, qc])
}
