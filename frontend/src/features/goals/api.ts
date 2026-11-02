import { api } from '@/shared/api/client'

export type Kind = 'financial' | 'habit' | 'shared' | 'project'
export type Status = 'active' | 'paused' | 'done' | 'archived'
export type Period = 'week' | 'month'

export type Progress = {
  done: number
  total: number
  /** inteiro, arredondado para baixo; passa de 100 em hábito e meta do casal (a barra limita, o dado fica) */
  percent: number
  unit: 'cents' | 'count' | 'steps'
  on_track?: boolean
  required_per_month_cents?: number
  /** economia do orçamento: o mês corrente ainda pode mudar */
  provisional?: boolean
  /** economia do orçamento com Finanças desligado */
  unavailable?: boolean
  suggest_complete?: boolean
  /** hábito e meta do casal: o período corrente, pontas incluídas */
  period?: { start: string; end: string }
  marked_today?: boolean
  /** meta do casal que exige os dois: dias com uma só marcação */
  pending_days?: string[]
}

export type Step = {
  id: string
  position: number
  title: string
  /** com tarefa, é o responsável da tarefa (nulo se recusada) */
  assignee_id: string | null
  due_on?: string
  /** com tarefa, vem da tarefa: a etapa nunca guarda estado próprio */
  done: boolean
  done_at?: string
  has_task: boolean
  task_id?: string
  task_status?: 'open' | 'done' | 'declined'
}

export type Contribution = {
  id: string
  amount_cents: number
  made_on: string
  note?: string
  created_by: string
  created_by_name: string
  created_at: string
}

/** `by`: ids de quem marcou, em ordem sem significado. */
export type DayMarks = { day: string; by: string[]; counted: boolean }
export type PeriodSummary = { start: string; end: string; count: number; target: number }

export type Goal = {
  id: string
  created_by: string
  owner_id: string | null
  scope: 'household' | 'personal'
  visibility: 'household' | 'private' | 'view'
  kind: Kind
  title: string
  /** ausente para quem só enxerga */
  notes?: string
  status: Status
  due_on: string | null
  completed_at?: string
  completed_by?: string
  version: number
  /** `view`: o parceiro lendo uma meta pessoal compartilhada */
  access: 'full' | 'view'
  can_edit: boolean
  created_at: string
  updated_at: string

  financial?: { target_cents: number; source: 'contributions' | 'budget_savings'; category_id?: string; baseline_on: string }
  habit?: { target_count: number; period: Period; weekdays: number; remind_min: number; remind_enabled: boolean }
  shared?: { target_count: number; period: Period; requires_both: boolean }
  steps?: Step[]

  progress: Progress
  recent?: { contributions?: Contribution[]; checkins?: DayMarks[] }
  /** a meta foi gravada, mas a tarefa de alguma etapa ainda será criada */
  sync_pending?: boolean
}

export type StepInput = { title: string; assignee_id?: string; due_on?: string; task?: boolean }

export type GoalInput = {
  kind: Kind
  scope: 'household' | 'personal'
  visibility?: 'private' | 'view'
  title: string
  notes?: string
  due_on?: string
  financial?: { target_cents: number; source?: 'contributions' | 'budget_savings'; category_id?: string; baseline_on?: string }
  habit?: { target_count: number; period: Period; weekdays?: number; remind_enabled?: boolean; remind_min?: number }
  shared?: { target_count: number; period: Period; requires_both?: boolean }
  steps?: StepInput[]
}

export type GoalUpdate = {
  version: number
  title: string
  notes?: string
  due_on?: string
  visibility?: 'private' | 'view'
  financial?: { target_cents: number }
  habit?: { target_count: number; weekdays: number; remind_enabled: boolean; remind_min?: number }
  shared?: { target_count: number; requires_both: boolean }
}

export type StepUpdate = { title?: string; assignee_id?: string; due_on?: string; position?: number }

export type GoalQuery = { scope?: 'household' | 'mine'; kind?: Kind; status?: Status | 'all' }
export type History = { from: string; to: string; days: DayMarks[]; periods: PeriodSummary[] }
export type ContributionPage = { items: Contribution[]; next_before?: string }

const qs = (params: Record<string, string | number | undefined>) => {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== '') q.set(k, String(v))
  const s = q.toString()
  return s ? `?${s}` : ''
}

const json = (method: string, body?: unknown) => ({ method, body })
const base = (id: string) => `/v1/goals/${id}`

/** Uma função por rota. O cliente nunca soma nada: toda conta vem em `progress`. */
export const goalsApi = {
  list: async (q: GoalQuery = {}) => (await api<Goal[] | null>(`/v1/goals${qs(q)}`)) ?? [],
  get: (id: string) => api<Goal>(base(id)),
  create: (v: GoalInput) => api<Goal>('/v1/goals', json('POST', v)),
  update: (id: string, v: GoalUpdate) => api<Goal>(base(id), json('PUT', v)),
  remove: (id: string) => api<void>(base(id), json('DELETE')),
  transition: (id: string, action: 'pause' | 'resume' | 'complete' | 'archive' | 'unarchive') =>
    api<Goal>(`${base(id)}/${action}`, json('POST')),

  contribute: (id: string, v: { amount_cents: number; made_on?: string; note?: string }) =>
    api<{ contribution: Contribution; goal: Goal }>(`${base(id)}/contributions`, json('POST', v)),
  contributions: async (id: string, before?: string) => {
    const page = await api<ContributionPage | null>(`${base(id)}/contributions${qs({ before, limit: 20 })}`)
    return { items: page?.items ?? [], next_before: page?.next_before }
  },
  removeContribution: (id: string, cid: string) => api<Goal>(`${base(id)}/contributions/${cid}`, json('DELETE')),

  checkin: (id: string, day?: string) => api<{ created: boolean; goal: Goal }>(`${base(id)}/checkins`, json('POST', day ? { day } : undefined)),
  undoCheckin: (id: string, day: string) => api<Goal>(`${base(id)}/checkins/${day}`, json('DELETE')),
  history: async (id: string, range: { from?: string; to?: string } = {}) => {
    const h = await api<History>(`${base(id)}/history${qs(range)}`)
    return { ...h, days: h.days ?? [], periods: h.periods ?? [] }
  },

  addStep: (id: string, v: StepInput) => api<Goal>(`${base(id)}/steps`, json('POST', v)),
  updateStep: (id: string, sid: string, v: StepUpdate) => api<Goal>(`${base(id)}/steps/${sid}`, json('PUT', v)),
  removeStep: (id: string, sid: string) => api<Goal>(`${base(id)}/steps/${sid}`, json('DELETE')),
  stepDone: (id: string, sid: string, done: boolean) => api<Goal>(`${base(id)}/steps/${sid}/${done ? 'done' : 'undone'}`, json('POST')),
  stepToTask: (id: string, sid: string) => api<Goal>(`${base(id)}/steps/${sid}/task`, json('POST')),
  stepDropTask: (id: string, sid: string) => api<Goal>(`${base(id)}/steps/${sid}/task`, json('DELETE')),
}

/** Link direto para o navegador baixar (sem fetch + blob). */
export const goalsExportUrl = (format: 'csv' | 'json' = 'csv') => `/v1/goals/export?format=${format}`
