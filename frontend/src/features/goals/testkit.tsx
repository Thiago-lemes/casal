import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { vi } from 'vitest'
import type { Goal } from './api'
import type { Ctx } from './types'

export const ME = { id: 'me', name: 'Ana', timezone: 'America/Sao_Paulo' }
export const BIA = { id: 'bia', name: 'Bia' }
export const ctx: Ctx = { me: ME, partner: BIA, people: [ME, BIA] }

export type Call = { method: string; url: string; body?: Record<string, unknown> }
type Reply = unknown | ((call: Call) => unknown)

/** Servidor falso: `"METHOD /v1/caminho"` (sem a query) → resposta. Sem rota, responde 404 `not_found`. */
export function fakeServer(routes: Record<string, Reply>) {
  const calls: Call[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init: { method?: string; body?: string } = {}) => {
      const method = init.method ?? 'GET'
      calls.push({ method, url, body: init.body ? JSON.parse(init.body) : undefined })
      const route = routes[`${method} ${url.split('?')[0]}`]
      if (route === undefined) return { ok: false, status: 404, json: async () => ({ code: 'not_found' }) }
      const out = typeof route === 'function' ? (route as (c: Call) => unknown)(calls[calls.length - 1]) : route
      if (out && typeof out === 'object' && 'status' in out && 'code' in out) {
        const e = out as { status: number; code: string }
        return { ok: false, status: e.status, json: async () => ({ code: e.code }) }
      }
      if (out === null) return { ok: true, status: 204, json: async () => null }
      return { ok: true, status: method === 'POST' ? 201 : 200, json: async () => out }
    }),
  )
  return { calls, sent: (method: string, path: string) => calls.filter((c) => c.method === method && c.url.split('?')[0] === path) }
}

export function renderWithQuery(ui: ReactElement, path = '/') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

export const goal = (over: Partial<Goal> = {}): Goal => ({
  id: 'g1', created_by: 'me', owner_id: 'me', scope: 'personal', visibility: 'private', kind: 'habit', title: 'Correr',
  status: 'active', due_on: null, version: 1, access: 'full', can_edit: true, created_at: '', updated_at: '',
  habit: { target_count: 3, period: 'week', weekdays: 0, remind_min: 540, remind_enabled: false },
  progress: { done: 1, total: 3, percent: 33, unit: 'count', marked_today: false, period: { start: '2026-11-02', end: '2026-11-08' } },
  ...over,
})
