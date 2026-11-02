import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Block, Occurrence } from './api'
import { canEditBlock, RoutinePage } from './RoutinePage'

const TZ = 'America/Sao_Paulo'
const me = { id: 'me', name: 'Ana', timezone: TZ }
const people = [me, { id: 'other', name: 'Bia' }]

// hoje, 09:00 e 14:00 no fuso (UTC-3)
const todayKey = new Intl.DateTimeFormat('en-CA', { timeZone: TZ }).format(new Date())
const at = (h: number, m = 0) => new Date(`${todayKey}T${String(h + 3).padStart(2, '0')}:${String(m).padStart(2, '0')}:00Z`).toISOString()

const occ = (over: Partial<Occurrence>): Occurrence => ({
  block_id: 'b1',
  owner_id: 'me',
  owner_name: 'Ana',
  title: 'Trabalho',
  category: 'work',
  location: 'Escritório',
  busy: false,
  start_at: at(9),
  end_at: at(12),
  exception: false,
  ...over,
})

const block = (over: Partial<Block>): Block => ({
  id: 'b1',
  owner_id: 'me',
  owner_name: 'Ana',
  title: 'Trabalho',
  category: 'work',
  location: '',
  visibility: 'private',
  busy: false,
  recurrence: 'weekly',
  weekdays: [1, 2, 3, 4, 5],
  interval_weeks: 1,
  start_date: '2026-01-05',
  end_date: null,
  start: '09:00',
  end: '12:00',
  version: 1,
  ...over,
})

function server(data: { blocks?: Block[]; occurrences?: Occurrence[]; free?: unknown[] }) {
  const calls: { method: string; url: string; body?: unknown }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init: { method?: string; body?: string } = {}) => {
      const method = init.method ?? 'GET'
      calls.push({ method, url, body: init.body ? JSON.parse(init.body) : undefined })
      const ok = (body: unknown, status = 200) => ({ ok: true, status, json: async () => body })
      if (url.includes('/routine/occurrences')) return ok(data.occurrences ?? [])
      if (url.includes('/routine/blocks') && method === 'GET') return ok(data.blocks ?? [])
      if (url.includes('/routine/now')) return ok([{ user_id: 'me', name: 'Ana', now: null }, { user_id: 'other', name: 'Bia', now: occ({ owner_id: 'other', owner_name: 'Bia', busy: true, title: '' }) }])
      if (url.includes('/routine/free')) return ok(data.free ?? [])
      return { ok: true, status: 204, json: async () => null }
    }),
  )
  return { calls }
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <RoutinePage me={me} people={people} />
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('canEditBlock', () => {
  it('o dono edita; do casal, os dois; "ocupado" do parceiro, ninguém', () => {
    expect(canEditBlock(block({}), 'me')).toBe(true)
    expect(canEditBlock(block({ owner_id: 'other', visibility: 'view' }), 'me')).toBe(false)
    expect(canEditBlock(block({ owner_id: 'other', visibility: 'household' }), 'me')).toBe(true)
    expect(canEditBlock(block({ owner_id: 'other', visibility: 'busy', busy: true }), 'me')).toBe(false)
    expect(canEditBlock(undefined, 'me')).toBe(false)
  })
})

describe('RoutinePage', () => {
  it('sem nenhum bloco: estado vazio que ensina a montar a rotina', async () => {
    server({})
    renderPage()
    expect(await screen.findByText('Monte a sua rotina')).toBeInTheDocument()
  })

  it('mostra "Ocupado" no horário do parceiro, sem título nem local', async () => {
    server({
      blocks: [block({})],
      occurrences: [occ({}), occ({ block_id: 'b2', owner_id: 'other', owner_name: 'Bia', busy: true, title: '', location: '', start_at: at(14), end_at: at(15) })],
    })
    renderPage()
    expect(await screen.findByRole('button', { name: /Trabalho/ })).toBeInTheDocument()
    expect(screen.getAllByText('Ocupado').length).toBeGreaterThan(0)
  })

  it('"quem está onde agora" mostra cada pessoa', async () => {
    server({ blocks: [block({})], occurrences: [occ({})] })
    renderPage()
    expect(await screen.findByText('Quem está onde agora')).toBeInTheDocument()
    expect(await screen.findByText('Livre agora')).toBeInTheDocument()
    expect(await screen.findByText(/Ocupado até/)).toBeInTheDocument()
  })

  it('"Não vou neste dia" abre uma exceção de folga no dia certo', async () => {
    const { calls } = server({ blocks: [block({})], occurrences: [occ({})] })
    renderPage()
    await userEvent.click(await screen.findByRole('button', { name: /Trabalho/ }))
    await userEvent.click(await screen.findByRole('button', { name: 'Não vou neste dia' }))
    await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true))
    const put = calls.find((c) => c.method === 'PUT')!
    expect(put.url).toBe(`/v1/routine/blocks/b1/exceptions/${todayKey}`)
    expect(put.body).toEqual({ skip: true })
  })

  it('bloco do parceiro só visível: mostra os detalhes mas não oferece exceção', async () => {
    server({
      blocks: [block({ id: 'b9', owner_id: 'other', owner_name: 'Bia', visibility: 'view', title: 'Yoga' })],
      occurrences: [occ({ block_id: 'b9', owner_id: 'other', owner_name: 'Bia', title: 'Yoga', category: 'workout' })],
    })
    renderPage()
    await userEvent.click(await screen.findByRole('button', { name: /Yoga/ }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('Bia')).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: 'Não vou neste dia' })).not.toBeInTheDocument()
  })

  it('a aba de horários livres lista as janelas e o vazio explica', async () => {
    server({ blocks: [block({})], occurrences: [], free: [{ start_at: at(14), end_at: at(16, 30) }] })
    renderPage()
    await userEvent.click(await screen.findByRole('tab', { name: 'Horários livres' }))
    expect(await screen.findByText('2 h 30 min')).toBeInTheDocument()
  })
})
