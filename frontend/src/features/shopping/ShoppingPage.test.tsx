import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Item } from './api'
import { ShoppingPage } from './ShoppingPage'

const item = (name: string, over: Partial<Item> = {}): Item => ({
  id: name,
  name,
  quantity: null,
  unit: '',
  category: 'grocery',
  recurring: false,
  source: 'manual',
  added_by: 'me',
  created_at: '2026-11-01T10:00:00Z',
  ...over,
})

/** Servidor falso da lista de compras: guarda os itens e responde às rotas que a página usa. */
function server(initial: Item[], reply: Partial<{ createMerged: boolean; restocked: number }> = {}) {
  let items = [...initial]
  const calls: { method: string; url: string; body?: unknown }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init: { method?: string; body?: string } = {}) => {
      const method = init.method ?? 'GET'
      const body = init.body ? JSON.parse(init.body) : undefined
      calls.push({ method, url, body })
      const ok = (data: unknown, status = 200) => ({ ok: true, status, json: async () => data })

      if (method === 'GET') return ok(items)
      if (url.endsWith('/restock')) return ok({ restocked: reply.restocked ?? 0 })
      if (url.endsWith('/bought')) {
        const id = url.split('/')[4]
        items = items.map((i) => (i.id === id ? { ...i, bought_at: '2026-11-01T12:00:00Z' } : i))
        return ok(items.find((i) => i.id === id))
      }
      if (url.endsWith('/unbought')) {
        const id = url.split('/')[4]
        items = items.map((i) => (i.id === id ? { ...i, bought_at: null } : i))
        return ok(items.find((i) => i.id === id))
      }
      if (method === 'POST') {
        const created = item(body.name, { ...body })
        if (!reply.createMerged) items = [...items, created]
        return ok({ ...created, merged: !!reply.createMerged }, reply.createMerged ? 200 : 201)
      }
      return { ok: true, status: 204, json: async () => null }
    }),
  )
  return { calls }
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <ShoppingPage />
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('ShoppingPage', () => {
  it('lista vazia: estado vazio que ensina o próximo passo', async () => {
    server([])
    renderPage()
    expect(await screen.findByText('A lista está vazia')).toBeInTheDocument()
  })

  it('agrupa por categoria, na ordem do mercado, e mostra a quantidade formatada', async () => {
    server([
      item('Sabão', { category: 'cleaning' }),
      item('Maçã', { category: 'produce', quantity: 1.5, unit: 'kg' }),
      item('Arroz', { category: 'grocery' }),
    ])
    renderPage()
    await screen.findByText('Maçã')
    const headings = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)
    expect(headings.slice(0, 4)).toEqual(['Adicionar à lista', 'Hortifrúti', 'Mercado', 'Limpeza'])
    expect(screen.getByText('1,5 kg')).toBeInTheDocument()
  })

  it('marcar como comprado leva o item para "comprados" e dá para desfazer', async () => {
    const { calls } = server([item('Leite')])
    renderPage()
    await userEvent.click(await screen.findByRole('button', { name: 'Marcar como comprado: Leite' }))

    const bought = (await screen.findByText('Comprados nas últimas 24 horas')).closest('section')!
    expect(within(bought).getByText('Leite')).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'POST' && c.url === '/v1/shopping/items/Leite/bought')).toBe(true)

    await userEvent.click(within(bought).getByRole('button', { name: 'Desfazer a compra: Leite' }))
    await waitFor(() => expect(screen.queryByText('Comprados nas últimas 24 horas')).not.toBeInTheDocument())
  })

  it('adicionar manda o item e, se já existia, avisa que somou a quantidade', async () => {
    const { calls } = server([item('Leite')], { createMerged: true })
    renderPage()
    await screen.findByText('Leite')
    await userEvent.type(screen.getByLabelText('O que falta?'), 'Leite')
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Leite já estava na lista: somamos à quantidade.')
    const post = calls.find((c) => c.method === 'POST' && c.url === '/v1/shopping/items')!
    expect(post.body).toMatchObject({ name: 'Leite', quantity: null, category: 'grocery', recurring: false })
  })

  it('quantidade digitada com vírgula vai como número', async () => {
    const { calls } = server([])
    renderPage()
    await screen.findByText('A lista está vazia')
    await userEvent.type(screen.getByLabelText('O que falta?'), 'Tomate')
    await userEvent.click(screen.getByRole('button', { name: 'Mais detalhes' }))
    await userEvent.type(screen.getByLabelText('Quantidade'), '1,5')
    await userEvent.type(screen.getByLabelText('Unidade'), 'kg')
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar' }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST')).toBe(true))
    expect(calls.find((c) => c.method === 'POST')!.body).toMatchObject({ name: 'Tomate', quantity: 1.5, unit: 'kg' })
  })

  it('quantidade inválida desabilita o botão e explica', async () => {
    server([])
    renderPage()
    await screen.findByText('A lista está vazia')
    await userEvent.type(screen.getByLabelText('O que falta?'), 'Tomate')
    await userEvent.click(screen.getByRole('button', { name: 'Mais detalhes' }))
    await userEvent.type(screen.getByLabelText('Quantidade'), 'muito')
    expect(screen.getByRole('button', { name: 'Adicionar' })).toBeDisabled()
    expect(screen.getByText('Use um número, como 2 ou 1,5.')).toBeInTheDocument()
  })

  it('repor recorrentes avisa quantos voltaram', async () => {
    server([item('Café', { recurring: true })], { restocked: 2 })
    renderPage()
    await userEvent.click(await screen.findByRole('button', { name: 'Repor recorrentes' }))
    expect(await screen.findByText('2 itens recorrentes voltaram à lista.')).toBeInTheDocument()
  })

  it('modo mercado esconde o cadastro e a edição, e deixa só marcar', async () => {
    server([item('Leite')])
    renderPage()
    await screen.findByText('Leite')
    expect(screen.getByLabelText('O que falta?')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Editar Leite' })).toBeInTheDocument()

    await userEvent.click(screen.getAllByRole('button', { name: 'Modo mercado' })[0])
    expect(screen.queryByLabelText('O que falta?')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Editar Leite' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Marcar como comprado: Leite' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Repor recorrentes' })).not.toBeInTheDocument()
  })
})
