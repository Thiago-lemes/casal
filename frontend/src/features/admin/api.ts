import { api } from '@/shared/api/client'

// O painel de suporte tem sessão própria (cookie admin_session, só em /admin). Toda chamada leva X-Requested-With:
// o servidor o exige nas escritas, e um formulário de outro site não consegue enviá-lo.
const HEADERS = { 'X-Requested-With': 'zoog-admin' }

function call<T>(path: string, method = 'GET', body?: unknown) {
  return api<T>(`/admin/v1${path}`, { method, body, headers: HEADERS })
}

export type AdminAccount = { id: string; email: string }
export type Person = { id: string; email: string; name: string }
export type HouseholdSummary = { id: string; created_at: string; members: Person[] }
/** Metadados e contagens do casal. Nunca o conteúdo: as contagens são o total do módulo, sem separar o que é pessoal. */
export type HouseholdDetail = HouseholdSummary & {
  modules: string[]
  last_used_at: string | null
  push_devices: number
  counts: Record<string, number>
}
export type ModuleState = { name: string; enabled: boolean }
export type UserDetail = Person & {
  created_at: string
  blocked_at: string | null
  household_id: string | null
  last_used_at: string | null
  push_devices: number
}
export type SignupCode = {
  id: string
  note: string
  expires_at: string
  max_uses: number
  uses: number
  revoked_at?: string | null
  created_at: string
}
export type FeedbackItem = {
  id: string
  household_id: string
  author: string
  message: string
  screen: string
  created_at: string
  read_at: string | null
}
export type AuditRow = {
  id: number
  occurred_at: string
  admin_email: string
  action: string
  target_type: string
  target_id: string | null
  before: Record<string, unknown> | null
  after: Record<string, unknown> | null
}

export const adminApi = {
  me: () => call<AdminAccount>('/me'),
  login: (email: string, password: string, code: string) => call<AdminAccount>('/auth/login', 'POST', { email, password, code }),
  logout: () => call<void>('/auth/logout', 'POST'),

  search: (q: string) => call<HouseholdSummary[]>(`/households?q=${encodeURIComponent(q)}`),
  household: (id: string) => call<{ household: HouseholdDetail; modules: ModuleState[] }>(`/households/${id}`),
  setModule: (id: string, name: string, enabled: boolean) =>
    call<{ modules: ModuleState[] }>(`/households/${id}/modules/${name}`, 'PUT', { enabled }),
  user: (id: string) => call<UserDetail>(`/users/${id}`),
  block: (id: string, blocked: boolean) => call<UserDetail>(`/users/${id}/${blocked ? 'block' : 'unblock'}`, 'POST'),

  codes: () => call<SignupCode[]>('/signup-codes'),
  createCode: (note: string, maxUses: number, days: number) =>
    call<{ code: string; signup_code: SignupCode }>('/signup-codes', 'POST', { note, max_uses: maxUses, days }),
  revokeCode: (id: string) => call<void>(`/signup-codes/${id}`, 'DELETE'),

  feedback: (unread: boolean) => call<FeedbackItem[]>(`/feedback${unread ? '?unread=true' : ''}`),
  readFeedback: (id: string) => call<void>(`/feedback/${id}/read`, 'POST'),

  audit: () => call<AuditRow[]>('/audit?limit=200'),
}
