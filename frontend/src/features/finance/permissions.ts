import type { Expense } from './api'

export type ExpenseActions = {
  edit: boolean
  remove: boolean
  changeVisibility: boolean
  /** gasto pessoal do parceiro que aparece em "Meus" por estar em `view`: só leitura */
  sharedByPartner: boolean
}

/**
 * Espelha as regras do servidor para esconder o botão que falharia.
 * O servidor é a autoridade (`can_edit` já considera dono e acerto).
 */
export function expenseActions(expense: Expense, meId: string): ExpenseActions {
  const personal = expense.scope === 'personal'
  return {
    edit: expense.can_edit,
    remove: expense.can_edit,
    changeVisibility: personal && expense.can_edit,
    sharedByPartner: personal && expense.created_by !== meId,
  }
}
