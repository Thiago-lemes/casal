import { describe, expect, it } from 'vitest'
import type { Expense } from './api'
import { expenseActions } from './permissions'

const expense = (over: Partial<Expense> = {}): Expense => ({
  id: 'e1', created_by: 'me', paid_by: 'me', scope: 'household', visibility: 'household', amount_cents: 1000,
  category_id: 'c1', spent_on: '2026-11-01', description: '', source: 'manual', settled: false, can_edit: true,
  version: 1, created_at: '', updated_at: '', ...over,
})

describe('expenseActions', () => {
  it('o dono edita e apaga um gasto aberto', () => {
    expect(expenseActions(expense(), 'me')).toEqual({ edit: true, remove: true, changeVisibility: false, sharedByPartner: false })
  })

  it('gasto pessoal do dono também muda a visibilidade', () => {
    const a = expenseActions(expense({ scope: 'personal', visibility: 'private' }), 'me')
    expect(a.changeVisibility).toBe(true)
    expect(a.sharedByPartner).toBe(false)
  })

  it('gasto pessoal do parceiro em "view" é só leitura', () => {
    const a = expenseActions(expense({ scope: 'personal', visibility: 'view', created_by: 'bia', can_edit: false }), 'me')
    expect(a).toEqual({ edit: false, remove: false, changeVisibility: false, sharedByPartner: true })
  })

  it('gasto acertado não tem ações, mesmo sendo do dono', () => {
    const a = expenseActions(expense({ settled: true, can_edit: false }), 'me')
    expect(a.edit || a.remove || a.changeVisibility).toBe(false)
  })
})
