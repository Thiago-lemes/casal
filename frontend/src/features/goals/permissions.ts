import type { Goal } from './api'

export type GoalActions = {
  edit: boolean
  pause: boolean
  resume: boolean
  complete: boolean
  archive: boolean
  unarchive: boolean
  delete: boolean
  /** aporte e retirada: só meta financeira medida por aportes */
  contribute: boolean
  /** marcar o dia: hábito e meta do casal */
  mark: boolean
  addStep: boolean
}

const none: GoalActions = {
  edit: false, pause: false, resume: false, complete: false, archive: false,
  unarchive: false, delete: false, contribute: false, mark: false, addStep: false,
}

/**
 * Espelha as regras do servidor para esconder o botão que falharia (o servidor é a autoridade).
 * Concluída ou arquivada: só reabrir / desarquivar e apagar; o resto responderia `goal_closed`.
 */
export function goalActions(goal: Goal): GoalActions {
  if (!goal.can_edit) return none
  if (goal.status === 'done') return { ...none, resume: true, delete: true }
  if (goal.status === 'archived') return { ...none, unarchive: true, delete: true }

  const open = goal.status === 'active'
  return {
    ...none,
    edit: true,
    pause: open,
    resume: !open,
    complete: true,
    archive: true,
    delete: true,
    contribute: goal.kind === 'financial' && goal.financial?.source !== 'budget_savings',
    mark: goal.kind === 'habit' || goal.kind === 'shared',
    addStep: goal.kind === 'project',
  }
}
