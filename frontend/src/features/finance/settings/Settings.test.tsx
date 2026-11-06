import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ctx, fakeServer, renderWithQuery } from '../testkit'
import { DivisionSettings } from './DivisionSettings'
import { IncomeField } from './IncomeField'

afterEach(() => vi.unstubAllGlobals())

const settings = (over: Record<string, unknown> = {}) => ({
  split_mode: 'equal',
  effective_split: { me: 5000, bia: 5000 },
  proportional: { available: false, consented_by: [], income_set: { me: true, bia: false } },
  ...over,
})

describe('divisão e consentimento', () => {
  it('o diálogo de consentimento aparece antes de qualquer chamada de consentimento', async () => {
    const server = fakeServer({
      'GET /v1/finance/settings': settings(),
      'POST /v1/finance/settings/proportional/consent': settings({ proportional: { available: false, consented_by: ['me'], income_set: { me: true, bia: true } } }),
    })
    renderWithQuery(<DivisionSettings ctx={ctx} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Concordar com a divisão proporcional' }))

    expect(await screen.findByText(/quem sabe a própria renda consegue calcular a renda do outro/)).toBeInTheDocument()
    expect(server.sent('POST', '/v1/finance/settings/proportional/consent')).toHaveLength(0)

    await userEvent.click(screen.getByRole('button', { name: 'Concordo' }))
    await waitFor(() => expect(server.sent('POST', '/v1/finance/settings/proportional/consent')).toHaveLength(1))
  })

  it('cancelar o diálogo não concorda com nada', async () => {
    const server = fakeServer({ 'GET /v1/finance/settings': settings() })
    renderWithQuery(<DivisionSettings ctx={ctx} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Concordar com a divisão proporcional' }))
    await userEvent.click(await screen.findByRole('button', { name: 'Cancelar' }))
    expect(server.sent('POST', '/v1/finance/settings/proportional/consent')).toHaveLength(0)
  })

  it('proporcional só é escolhível quando disponível; mostra só quem informou a renda, nunca valores', async () => {
    fakeServer({ 'GET /v1/finance/settings': settings() })
    renderWithQuery(<DivisionSettings ctx={ctx} />)
    expect(await screen.findByText('Falta Bia informar a renda.')).toBeInTheDocument()
    expect(screen.queryByRole('radio', { name: 'Proporcional à renda' })).not.toBeInTheDocument()
  })

  it('com tudo pronto, oferece proporcional e mostra a divisão resultante', async () => {
    fakeServer({
      'GET /v1/finance/settings': settings({
        proportional: { available: true, split: { me: 6000, bia: 4000 }, consented_by: ['me', 'bia'], income_set: { me: true, bia: true } },
      }),
    })
    renderWithQuery(<DivisionSettings ctx={ctx} />)
    await userEvent.click(await screen.findByRole('radio', { name: 'Proporcional à renda' }))
    expect(screen.getByText(/Eu 60% \/ Bia 40%/)).toBeInTheDocument()
  })

  it('muda a divisão padrão para igual/combinada pelo PUT', async () => {
    const server = fakeServer({ 'GET /v1/finance/settings': settings(), 'PUT /v1/finance/settings': settings({ split_mode: 'fixed' }) })
    renderWithQuery(<DivisionSettings ctx={ctx} />)
    await userEvent.click(await screen.findByRole('radio', { name: 'Combinada' }))
    await userEvent.click(screen.getByRole('button', { name: 'Salvar divisão' }))
    await waitFor(() => expect(server.sent('PUT', '/v1/finance/settings')).toHaveLength(1))
    const body = server.sent('PUT', '/v1/finance/settings')[0].body as { split_mode: string; fixed_split: Record<string, number> }
    expect(body.split_mode).toBe('fixed')
    expect(Object.values(body.fixed_split).reduce((a, b) => a + b, 0)).toBe(10000)
  })
})

describe('renda', () => {
  it('só a própria renda: nunca renderiza valor de outra pessoa', async () => {
    const server = fakeServer({
      'GET /v1/finance/income': { monthly_income_cents: 550000 },
      // o servidor só diz QUEM informou; qualquer valor aqui seria um vazamento
      'GET /v1/finance/settings': settings({ proportional: { available: false, consented_by: [], income_set: { me: true, bia: true } } }),
    })
    renderWithQuery(
      <>
        <DivisionSettings ctx={ctx} />
        <IncomeField />
      </>,
    )
    expect(await screen.findByLabelText('Renda mensal (opcional)')).toHaveValue('5500')
    expect(await screen.findByText('Bia informou a renda.')).toBeInTheDocument()
    // o único valor na tela é a própria renda, lida de volta sob o campo
    const text = (document.body.textContent ?? '').replace(/Entendi R\$\s?[\d.,]+\./, '')
    expect(text).not.toMatch(/R\$\s?\d/)
    // a única rota de renda é a da própria pessoa
    expect(server.calls.filter((c) => c.url.includes('/income')).every((c) => c.url === '/v1/finance/income')).toBe(true)
  })

  it('salva a renda em centavos', async () => {
    const server = fakeServer({
      'GET /v1/finance/income': { monthly_income_cents: null },
      'GET /v1/finance/settings': settings(),
      'PUT /v1/finance/income': { monthly_income_cents: 450050 },
    })
    renderWithQuery(<IncomeField />)
    await userEvent.type(await screen.findByLabelText('Renda mensal (opcional)'), '4.500,50')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar renda' }))
    await waitFor(() => expect(server.sent('PUT', '/v1/finance/income')).toHaveLength(1))
    expect(server.sent('PUT', '/v1/finance/income')[0].body).toEqual({ monthly_income_cents: 450050 })
  })

  it('avisa que o parceiro será notificado quando a divisão está proporcional', async () => {
    fakeServer({
      'GET /v1/finance/income': { monthly_income_cents: 100000 },
      'GET /v1/finance/settings': settings({ split_mode: 'proportional' }),
    })
    renderWithQuery(<IncomeField />)
    expect(await screen.findByText(/a outra pessoa será avisada/)).toBeInTheDocument()
  })
})
