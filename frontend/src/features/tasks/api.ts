import { api } from '@/shared/api/client'

export type Repeat = 'none' | 'daily' | 'weekly' | 'monthly'

export type TaskStatus = 'open' | 'done' | 'declined' | 'canceled'

export type Task = {
  id: string
  created_by: string
  created_by_name: string
  /** nulo = tarefa do casal */
  assignee_id: string | null
  title: string
  due_at?: string
  status: TaskStatus
  decline_reason?: string
  completed_by?: string
  completed_at?: string
  /** `none`, `daily`, `weekly` ou `monthly`; ao concluir, a próxima já nasce aberta */
  repeat: Repeat
  /** rodízio: a cada ciclo o responsável passa para a outra pessoa */
  rotation: boolean
  created_at: string
}

/** O backend não lista canceladas: um status por chamada, só estes três. */
export type ListStatus = 'open' | 'done' | 'declined'

/** Quem fica com a tarefa. `assignee_id` vazio + `shared` = do casal. */
export type Assignment = { assignee_id: string; shared: boolean }

export const tasksApi = {
  list: async (status: ListStatus) => (await api<Task[] | null>(`/v1/tasks?status=${status}`)) ?? [],
  create: (v: Assignment & { title: string; due_at?: string; repeat?: Repeat; rotation?: boolean }) => api<Task>('/v1/tasks', { method: 'POST', body: v }),
  complete: (id: string) => api<Task>(`/v1/tasks/${id}/complete`, { method: 'POST' }),
  decline: (id: string, reason: string) =>
    api<Task>(`/v1/tasks/${id}/decline`, { method: 'POST', body: { reason } }),
  assign: (id: string, to: Assignment) => api<Task>(`/v1/tasks/${id}/assign`, { method: 'POST', body: to }),
  cancel: (id: string) => api<void>(`/v1/tasks/${id}`, { method: 'DELETE' }),
}
