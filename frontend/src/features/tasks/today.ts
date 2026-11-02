import type { Task } from './api'
import { sortOpen } from './permissions'

/**
 * "Tarefas de hoje": abertas que vencem até o fim de hoje (inclui as atrasadas) ou não têm prazo,
 * e que são minhas ou do casal. Com prazo primeiro, como na página de tarefas.
 */
export function tasksForToday(open: Task[], meId: string, now = new Date()): Task[] {
  const endOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1)
  return sortOpen(
    open.filter((t) => {
      if (t.status !== 'open') return false
      const mineOrShared = t.assignee_id === null || t.assignee_id === meId
      const dueOk = !t.due_at || new Date(t.due_at) < endOfToday
      return mineOrShared && dueOk
    }),
  )
}
