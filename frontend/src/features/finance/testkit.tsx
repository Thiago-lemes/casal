import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { vi } from 'vitest'
import type { Ctx } from './types'

export const ME = { id: 'me', name: 'Ana', timezone: 'America/Sao_Paulo' }
export const BIA = { id: 'bia', name: 'Bia' }
export const ctx: Ctx = { me: ME, partner: BIA, people: [ME, BIA] }
export const solo: Ctx = { me: ME, people: [ME] }

export const categories = [
  { id: 'c-groc', name: 'Mercado', key: 'groceries', archived: false },
  { id: 'c-home', name: 'Moradia', key: 'housing', archived: false },
  { id: 'c-old', name: 'Antiga', archived: true },
]

export type Call = { method: string; url: string; body?: Record<string, unknown> }
type Reply = unknown | ((call: Call) => unknown | Response)
type Routes = Record<string, Reply>

/** Servidor falso: `"METHOD /v1/caminho"` (sem a query) → resposta. Sem rota, responde 404 `not_found`. */
export function fakeServer(routes: Routes) {
  const calls: Call[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init: { method?: string; body?: string } = {}) => {
      const method = init.method ?? 'GET'
      const call: Call = { method, url, body: init.body ? JSON.parse(init.body) : undefined }
      calls.push(call)
      const key = `${method} ${url.split('?')[0]}`
      const route = routes[key] ?? (key === 'GET /v1/finance/categories' ? categories : undefined)
      if (route === undefined) return { ok: false, status: 404, json: async () => ({ code: 'not_found' }) }
      const out = typeof route === 'function' ? (route as (c: Call) => unknown)(call) : route
      if (out && typeof out === 'object' && 'status' in (out as object) && 'code' in (out as object)) {
        const e = out as { status: number; code: string }
        return { ok: false, status: e.status, json: async () => ({ code: e.code }) }
      }
      if (out === null) return { ok: true, status: 204, json: async () => null }
      return { ok: true, status: method === 'POST' ? 201 : 200, json: async () => out }
    }),
  )
  return { calls, sent: (method: string, path: string) => calls.filter((c) => c.method === method && c.url.split('?')[0] === path) }
}

export function renderWithQuery(ui: ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

export const expense = (over: Record<string, unknown> = {}) => ({
  id: 'e1', created_by: 'me', paid_by: 'me', scope: 'household', visibility: 'household', amount_cents: 10000,
  category_id: 'c-groc', spent_on: '2026-11-01', description: 'Feira', source: 'manual', settled: false,
  can_edit: true, version: 1, created_at: '', updated_at: '', ...over,
})
