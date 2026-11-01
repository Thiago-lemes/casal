import { api } from '@/shared/api/client'

export type Member = { id: string; name: string; joined_at: string }
export type Household = { id: string; members: Member[] }

export const householdApi = {
  get: () => api<Household>('/v1/household'),
  create: () => api<{ id: string }>('/v1/household', { method: 'POST' }),
  invite: () => api<{ token: string; expires_at: string }>('/v1/household/invites', { method: 'POST' }),
  accept: (token: string) =>
    api<{ household_id: string }>('/v1/household/invites/accept', { method: 'POST', body: { token } }),
}
