import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Note } from './api'
import { NotesPage } from './NotesPage'

const note = (id: string, over: Partial<Note>): Note => ({
  id,
  owner_id: 'me',
  owner_name: 'Ana',
  title: id,
  kind: 'text',
  body: '',
  items: null,
  tags: null,
  visibility: 'private',
  pinned: false,
  version: 1,
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-01T10:00:00Z',
  ...over,
})

function server(initial: Note[]) {
  const puts: unknown[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (_url: string, init: { method?: string; body?: string } = {}) => {
      if (init.method === 'PUT') {
        puts.push(JSON.parse(init.body!))
        return { ok: true, status: 200, json: async () => initial[0] }
      }
      return { ok: true, status: 200, json: async () => initial }
    }),
  )
  return { puts }
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <NotesPage me={{ id: 'me' }} />
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('NotesPage', () => {
  it('sem anotações: estado vazio que ensina o próximo passo', async () => {
    server([])
    renderPage()
    expect(await screen.findByText('Nenhuma anotação ainda')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Nova anotação' }).length).toBeGreaterThan(0)
  })

  it('fixadas aparecem primeiro e a busca filtra sem diferenciar acento', async () => {
    server([note('Feijão', { updated_at: '2026-11-02T00:00:00Z' }), note('Viagem', { pinned: true }), note('Mercado', { body: 'açúcar' })])
    renderPage()
    await screen.findByText('Viagem')
    const titles = screen.getAllByRole('button', { name: /^(Feijão|Viagem|Mercado)$/ }).map((b) => b.textContent)
    expect(titles[0]).toBe('Viagem')

    await userEvent.type(screen.getByLabelText('Buscar'), 'acucar')
    expect(screen.getByText('Mercado')).toBeInTheDocument()
    expect(screen.queryByText('Viagem')).not.toBeInTheDocument()

    await userEvent.clear(screen.getByLabelText('Buscar'))
    await userEvent.type(screen.getByLabelText('Buscar'), 'zzz')
    expect(await screen.findByText('Nada encontrado')).toBeInTheDocument()
  })

  it('marcar um item no cartão salva a lista inteira com a versão e sem mexer na visibilidade', async () => {
    const { puts } = server([
      note('Compras', { kind: 'checklist', items: [{ text: 'Leite', done: false }, { text: 'Pão', done: false }], version: 7 }),
    ])
    renderPage()
    const card = (await screen.findByText('Compras')).closest('li')!
    await userEvent.click(within(card).getByRole('checkbox', { name: 'Leite' }))

    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0]).toMatchObject({
      items: [{ text: 'Leite', done: true }, { text: 'Pão', done: false }],
      visibility: '',
      version: 7,
    })
  })

  it('anotação do parceiro de leitura: sem fixar e sem marcar itens', async () => {
    server([note('Dele', { owner_id: 'other', owner_name: 'Bia', visibility: 'view', kind: 'checklist', items: [{ text: 'X', done: false }] })])
    renderPage()
    const card = (await screen.findByText('Dele')).closest('li')!
    expect(within(card).getByRole('checkbox', { name: 'X' })).toBeDisabled()
    expect(within(card).getByRole('button', { name: /Fixar/ })).toBeDisabled()
    expect(within(card).getByText('de Bia')).toBeInTheDocument()
  })
})
