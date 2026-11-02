import type { Assignment, Task } from './api'

export type TaskActions = {
  complete: boolean
  decline: boolean
  /** o criador pega para si uma tarefa que está com outra pessoa ou foi recusada */
  assume: boolean
  /** o criador passa a tarefa para outra pessoa ou para o casal */
  reassign: boolean
  cancel: boolean
}

/**
 * Espelha as regras do backend só para esconder botões que falhariam.
 * O backend continua sendo a autoridade: se a regra mudar lá, a tela perde um botão, nunca abre uma brecha.
 */
export function taskActions(task: Task, meId: string): TaskActions {
  const open = task.status === 'open'
  const creator = task.created_by === meId
  const mine = task.assignee_id === meId
  const editable = creator && (open || task.status === 'declined')

  return {
    complete: open && (task.assignee_id === null || mine || creator),
    decline: open && mine && !creator,
    assume: editable && !mine,
    reassign: editable,
    cancel: editable,
  }
}

export type Choice = 'me' | 'partner' | 'shared'

/** Converte a escolha da tela (eu, parceiro, do casal) no corpo que o backend espera. */
export function toAssignment(choice: Choice, meId: string, partnerId?: string): Assignment {
  if (choice === 'shared') return { assignee_id: '', shared: true }
  return { assignee_id: choice === 'partner' && partnerId ? partnerId : meId, shared: false }
}

/** Abertas: com prazo primeiro (mais próximo antes), depois as sem prazo (mais antigas antes). */
export function sortOpen(tasks: Task[]): Task[] {
  return [...tasks].sort((a, b) => {
    if (a.due_at && b.due_at) return a.due_at.localeCompare(b.due_at)
    if (a.due_at) return -1
    if (b.due_at) return 1
    return a.created_at.localeCompare(b.created_at)
  })
}

/** Concluídas: mais recentes primeiro. */
export function sortDone(tasks: Task[]): Task[] {
  return [...tasks].sort((a, b) => (b.completed_at ?? '').localeCompare(a.completed_at ?? ''))
}
