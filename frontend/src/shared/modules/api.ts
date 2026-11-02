import { api } from '@/shared/api/client'

export type ModuleState = { name: string; enabled: boolean; enabled_at?: string }

export const modulesApi = {
  list: async () => (await api<ModuleState[] | null>('/v1/modules')) ?? [],
  set: (name: string, enabled: boolean) => api<ModuleState>(`/v1/modules/${name}`, { method: 'PUT', body: { enabled } }),
}
