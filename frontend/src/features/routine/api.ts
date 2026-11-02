import { api } from '@/shared/api/client'

export type Category = 'work' | 'study' | 'workout' | 'commute' | 'appointment' | 'home' | 'other'
export const CATEGORIES: Category[] = ['work', 'study', 'workout', 'commute', 'appointment', 'home', 'other']

/** `private` o parceiro não vê; `busy` vê só o horário; `view` vê os detalhes; `household` bloco do casal, os dois editam. */
export type BlockVisibility = 'private' | 'busy' | 'view' | 'household'

export type Block = {
  id: string
  owner_id: string
  owner_name: string
  title: string
  category: Category
  location: string
  visibility: BlockVisibility
  /** verdadeiro: o parceiro marcou só "ocupado"; título, categoria e local chegam vazios */
  busy: boolean
  recurrence: 'once' | 'weekly'
  /** 1 segunda a 7 domingo */
  weekdays: number[] | null
  /** 1 a 4; 2 é semana sim, semana não */
  interval_weeks: number
  start_date: string
  end_date?: string | null
  /** "HH:MM"; `end` aceita "24:00" */
  start: string
  end: string
  version: number
  /** ids de outros blocos seus que se sobrepõem (aviso, nunca impede); só em `POST` e `PUT` */
  overlaps?: string[]
}

export type BlockInput = {
  title: string
  category: Category
  location: string
  visibility: BlockVisibility
  recurrence: 'once' | 'weekly'
  weekdays: number[]
  interval_weeks: number
  start_date: string
  /** vazio = sem fim */
  end_date: string
  start: string
  end: string
}

/** Uma ocorrência de um bloco num dia, já em instantes UTC. */
export type Occurrence = {
  block_id: string
  owner_id: string
  owner_name: string
  title: string
  category: Category
  location: string
  busy: boolean
  start_at: string
  end_at: string
  /** o dia tem uma exceção (outro horário) */
  exception: boolean
}

export type Presence = { user_id: string; name: string; now: Occurrence | null }
export type FreeWindow = { start_at: string; end_at: string }
export type ExceptionBody = { skip: true } | { start: string; end: string }

export const routineApi = {
  blocks: async () => (await api<Block[] | null>('/v1/routine/blocks')) ?? [],
  get: (id: string) => api<Block>(`/v1/routine/blocks/${id}`),
  create: (v: BlockInput) => api<Block>('/v1/routine/blocks', { method: 'POST', body: v }),
  update: (id: string, v: BlockInput & { version: number }) => api<Block>(`/v1/routine/blocks/${id}`, { method: 'PUT', body: v }),
  remove: (id: string) => api<void>(`/v1/routine/blocks/${id}`, { method: 'DELETE' }),
  setException: (id: string, date: string, body: ExceptionBody) =>
    api<void>(`/v1/routine/blocks/${id}/exceptions/${date}`, { method: 'PUT', body }),
  clearException: (id: string, date: string) => api<void>(`/v1/routine/blocks/${id}/exceptions/${date}`, { method: 'DELETE' }),
  /** `from` e `to` são dias AAAA-MM-DD (até 62), no fuso do perfil. */
  occurrences: async (from: string, to: string) =>
    (await api<Occurrence[] | null>(`/v1/routine/occurrences?from=${from}&to=${to}`)) ?? [],
  /** Janelas em que os dois estão livres no dia, com no mínimo `min` minutos. */
  free: async (date: string, min: number, fromHour = 7, toHour = 22) =>
    (await api<FreeWindow[] | null>(`/v1/routine/free?date=${date}&min=${min}&from_hour=${fromHour}&to_hour=${toHour}`)) ?? [],
  now: async () => (await api<Presence[] | null>('/v1/routine/now')) ?? [],
}
