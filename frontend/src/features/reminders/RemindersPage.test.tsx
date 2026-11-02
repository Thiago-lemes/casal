import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Reminder } from './api'
import { RemindersPage } from './RemindersPage'

const me = { id: 'me', timezone: 'America/Sao_Paulo' }
const partner = { id: 'other', name: 'Bia' }

const reminder = (id: string, over: Partial<Reminder>): Reminder => ({
  id,
  created_by: 'me',
  created_by_name: 'Ana',
  recipient_id: 'me',
  body: id,
  remind_at: '2026-11-02T20:00:00Z',
  status: 'scheduled',
  ...over,
})

function renderPage(data: Reminder[]) {
  vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, status: 200, json: async () => data })))
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <RemindersPage me={me} partner={partner} />
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('RemindersPage', () => {
  it('sem lembretes: estado vazio que ensina o próximo passo', async () => {
    renderPage([])
    expect(await screen.findByText('Nenhum lembrete pendente')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Novo lembrete' }).length).toBeGreaterThan(0)
  })

  it('pendentes por horário; resolvidos na outra aba, do mais recente ao mais antigo', async () => {
    renderPage([
      reminder('Depois', { remind_at: '2026-11-03T20:00:00Z' }),
      reminder('Antes', { remind_at: '2026-11-02T20:00:00Z' }),
      reminder('Velho', { status: 'resolved', resolved_at: '2026-11-01T10:00:00Z' }),
      reminder('Novo', { status: 'resolved', resolved_at: '2026-11-01T15:00:00Z' }),
    ])
    await screen.findByText('Antes')
    const order = screen.getAllByText(/^(Antes|Depois)$/).map((n) => n.textContent)
    expect(order).toEqual(['Antes', 'Depois'])
    expect(screen.queryByText('Novo')).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('tab', { name: 'Resolvidos' }))
    const resolved = screen.getAllByText(/^(Novo|Velho)$/).map((n) => n.textContent)
    expect(resolved).toEqual(['Novo', 'Velho'])
    expect(screen.queryByRole('button', { name: 'Dar baixa' })).not.toBeInTheDocument() // resolvidos são só leitura
  })

  it('a aba de resolvidos também tem estado vazio', async () => {
    renderPage([reminder('A', {})])
    await screen.findByText('A')
    await userEvent.click(screen.getByRole('tab', { name: 'Resolvidos' }))
    expect(screen.getByText('Nada resolvido ainda')).toBeInTheDocument()
  })
})
