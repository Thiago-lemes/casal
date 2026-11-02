import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { GoalsPage, splitMine } from './GoalsPage'
import { BIA, ctx, fakeServer, goal, renderWithQuery } from './testkit'

afterEach(() => vi.unstubAllGlobals())

const on = [{ name: 'goals', enabled: true }]
const page = (path = '/goals') => renderWithQuery(<GoalsPage ctx={ctx} />, path)

describe('GoalsPage', () => {
  it('módulo desligado: convite para ligar, sem abas e sem erro', async () => {
    const server = fakeServer({
      'GET /v1/modules': [{ name: 'goals', enabled: false }],
      'PUT /v1/modules/goals': { name: 'goals', enabled: true },
    })
    page()
    expect(await screen.findByText('Ligue Metas para começar')).toBeInTheDocument()
    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Ligar Metas' }))
    await waitFor(() => expect(server.sent('PUT', '/v1/modules/goals')).toHaveLength(1))
    expect(server.sent('PUT', '/v1/modules/goals')[0].body).toEqual({ enabled: true })
  })

  it('403 module_disabled vira o convite, não uma falha genérica', async () => {
    let modules = 0
    fakeServer({
      'GET /v1/modules': () => [{ name: 'goals', enabled: ++modules === 1 }],
      'GET /v1/goals': { status: 403, code: 'module_disabled' },
    })
    page()
    expect(await screen.findByText('Ligue Metas para começar')).toBeInTheDocument()
  })

  it('sem metas: explica os quatro tipos e oferece criar a primeira', async () => {
    fakeServer({ 'GET /v1/modules': on, 'GET /v1/goals': [] })
    page()
    expect(await screen.findByText('Nenhuma meta ainda')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Criar a primeira meta' })).toBeInTheDocument()
    expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Do casal', 'Minhas'])
  })

  it('o cartão mostra o progresso do servidor e "Pronta para concluir" sem concluir nada', async () => {
    const server = fakeServer({
      'GET /v1/modules': on,
      'GET /v1/goals': [
        goal({ id: 'a', title: 'Jantar fora', kind: 'shared', scope: 'household', owner_id: null, visibility: 'household', progress: { done: 1, total: 1, percent: 100, unit: 'count', suggest_complete: true } }),
        goal({ id: 'b', title: 'Alongar', status: 'paused' }),
      ],
    })
    page()
    expect(await screen.findByRole('link', { name: /Jantar fora/ })).toHaveAttribute('href', '/goals/a')
    expect(screen.getByText('Pronta para concluir')).toBeInTheDocument()
    expect(screen.getByText('Pausada')).toBeInTheDocument()
    expect(server.sent('POST', '/v1/goals/a/complete')).toHaveLength(0)
    expect(server.calls.find((c) => c.url.startsWith('/v1/goals?'))?.url).toContain('scope=household')
  })

  it('"Minhas" separa as pessoais das que o parceiro compartilhou (só leitura), sem escopo na consulta', async () => {
    const server = fakeServer({
      'GET /v1/modules': on,
      'GET /v1/goals': [
        goal({ id: 'own', title: 'Correr' }),
        goal({ id: 'view', title: 'Yoga', owner_id: 'bia', created_by: 'bia', visibility: 'view', access: 'view', can_edit: false }),
        goal({ id: 'home', title: 'Casa', scope: 'household', owner_id: null, visibility: 'household' }),
      ],
    })
    page()
    await userEvent.click(await screen.findByRole('tab', { name: 'Minhas' }))
    expect(await screen.findByRole('link', { name: /Correr/ })).toBeInTheDocument()
    const shared = await screen.findByRole('region', { name: 'Compartilhadas pelo seu par' })
    expect(within(shared).getByRole('link', { name: /Yoga/ })).toBeInTheDocument()
    expect(within(shared).getByText('Compartilhada por Bia')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Casa/ })).not.toBeInTheDocument()
    expect(server.calls.filter((c) => c.url.startsWith('/v1/goals?')).some((c) => c.url.includes('scope=mine'))).toBe(false)
  })

  it('splitMine: do casal fica de fora; view vai para as compartilhadas', () => {
    const r = splitMine([goal({ id: '1' }), goal({ id: '2', access: 'view', owner_id: BIA.id }), goal({ id: '3', scope: 'household' })])
    expect(r.own.map((g) => g.id)).toEqual(['1'])
    expect(r.shared.map((g) => g.id)).toEqual(['2'])
  })

  it('criar um hábito: o escopo some (só pessoal), "quem pode ver" começa privado e o corpo vai certo', async () => {
    const server = fakeServer({
      'GET /v1/modules': on,
      'GET /v1/goals': [],
      'POST /v1/goals': goal({ id: 'novo' }),
    })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Nova meta' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: /Hábito/ }))
    expect(within(dialog).queryByText('Do casal')).not.toBeInTheDocument()

    await userEvent.type(within(dialog).getByLabelText('Título'), 'Correr')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Continuar' }))
    expect(within(dialog).getByRole('radio', { name: 'Só eu' })).toBeChecked()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Criar meta' }))

    await waitFor(() => expect(server.sent('POST', '/v1/goals')).toHaveLength(1))
    expect(server.sent('POST', '/v1/goals')[0].body).toEqual({
      kind: 'habit', scope: 'personal', visibility: 'private', title: 'Correr',
      habit: { target_count: 3, period: 'week', weekdays: 0, remind_enabled: false },
    })
  })

  it('título vazio não envia e mostra o erro no campo', async () => {
    const server = fakeServer({ 'GET /v1/modules': on, 'GET /v1/goals': [] })
    page('/goals?new=1')
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: /Projeto/ }))
    await userEvent.click(within(dialog).getByRole('button', { name: /^Do casal/ }))
    await userEvent.click(within(dialog).getByRole('button', { name: 'Criar meta' }))
    expect(await within(dialog).findByText('O título precisa ter de 1 a 120 caracteres.')).toBeInTheDocument()
    expect(server.sent('POST', '/v1/goals')).toHaveLength(0)
  })
})
