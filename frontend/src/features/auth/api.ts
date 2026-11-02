import { api } from '@/shared/api/client'

export type User = { id: string; email: string; name: string; timezone: string }

export const authApi = {
  me: () => api<User>('/v1/me'),
  login: (email: string, password: string) => api<User>('/v1/auth/login', { method: 'POST', body: { email, password } }),
  register: (email: string, name: string, password: string) =>
    api<User>('/v1/auth/register', { method: 'POST', body: { email, name, password } }),
  update: (patch: { name?: string; timezone?: string }) => api<User>('/v1/me', { method: 'PATCH', body: patch }),
  logout: () => api<void>('/v1/auth/logout', { method: 'POST' }),
}
