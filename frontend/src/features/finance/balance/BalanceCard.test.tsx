import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ctx, expense, fakeServer, renderWithQuery, solo } from '../testkit'
import { BalanceCard } from './BalanceCard'
import { standing } from './balance'

afterEach(() => vi.unstubAllGlobals())

const balance = (over: Record<string, unknown> = {}) => ({
  through: '2026-11-02', from_user: 'me', to_user: 'bia', amount_cents: 15000, paid_cents: { me: 5000, bia: 20000 }, expenses: [expense()], ...over,
})

describe('standing', () => {
  it('devo, me devem e em dia', () => {
    expect(standing({ from_user: 'me', to_user: 'bia', amount_cents: 100 }, 'me')).toEqual({ kind: 'iOwe', to: 'bia', cents: 100 })
    expect(standing({ from_user: 'bia', to_user: 'me', amount_cents: 100 }, 'me')).toEqual({ kind: 'owedToMe', from: 'bia', cents: 100 })
    expect(standing({ amount_cents: 0 }, 'me')).toEqual({ kind: 'even' })
    expect(standing({ from_user: 'me', to_user: 'bia', amount_cents: 0 }, 'me')).toEqual({ kind: 'even' })
  })
})

describe('aba Saldo', () => {
  it('"Você deve R$ 150,00 para Bia"', async () => {
    fakeServer({ 'GET /v1/finance/balance': balance() })
    renderWithQuery(<BalanceCard ctx={ctx} />)
    expect(await screen.findByText(/Você deve R\$\s150,00 para Bia/)).toBeInTheDocument()
  })

  it('"Bia deve R$ 150,00 para você"', async () => {
    fakeServer({ 'GET /v1/finance/balance': balance({ from_user: 'bia', to_user: 'me' }) })
    renderWithQuery(<BalanceCard ctx={ctx} />)
    expect(await screen.findByText(/Bia deve R\$\s150,00 para você/)).toBeInTheDocument()
  })

  it('"Tudo em dia" não oferece acertar', async () => {
    fakeServer({ 'GET /v1/finance/balance': balance({ from_user: undefined, to_user: undefined, amount_cents: 0 }) })
    renderWithQuery(<BalanceCard ctx={ctx} />)
    expect(await screen.findByText('Tudo em dia')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Acertar' })).not.toBeInTheDocument()
  })

  it('casal de uma pessoa só explica que o acerto precisa de duas', async () => {
    const server = fakeServer({})
    renderWithQuery(<BalanceCard ctx={solo} />)
    expect(await screen.findByText('O acerto precisa de duas pessoas')).toBeInTheDocument()
    expect(server.sent('GET', '/v1/finance/balance')).toHaveLength(0)
  })

  it('acertar envia exatamente o valor e o dia que a pessoa está vendo', async () => {
    const server = fakeServer({
      'GET /v1/finance/balance': balance(),
      'POST /v1/finance/settlements': { id: 's1' },
    })
    renderWithQuery(<BalanceCard ctx={ctx} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Acertar' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/O app só registra/)).toBeInTheDocument()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Registrar acerto' }))

    await waitFor(() => expect(server.sent('POST', '/v1/finance/settlements')).toHaveLength(1))
    expect(server.sent('POST', '/v1/finance/settlements')[0].body).toEqual({ through_on: '2026-11-02', amount_cents: 15000 })
  })

  it('settlement_stale recarrega o saldo e NÃO reenvia sozinho com o valor novo', async () => {
    let gets = 0
    const server = fakeServer({
      'GET /v1/finance/balance': () => balance({ amount_cents: ++gets === 1 ? 15000 : 18000 }),
      'POST /v1/finance/settlements': { status: 409, code: 'settlement_stale' },
    })
    renderWithQuery(<BalanceCard ctx={ctx} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Acertar' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Registrar acerto' }))

    expect(await within(dialog).findByText(/O saldo mudou desde que você abriu esta tela/)).toBeInTheDocument()
    expect(await within(dialog).findByText(/180,00/)).toBeInTheDocument()
    // ficou esperando a confirmação: um só envio, com o valor antigo
    expect(server.sent('POST', '/v1/finance/settlements')).toHaveLength(1)
    expect(server.sent('POST', '/v1/finance/settlements')[0].body).toMatchObject({ amount_cents: 15000 })

    server.calls.length = 0
    await userEvent.click(within(dialog).getByRole('button', { name: 'Registrar acerto' }))
    await waitFor(() => expect(server.sent('POST', '/v1/finance/settlements')).toHaveLength(1))
    expect(server.sent('POST', '/v1/finance/settlements')[0].body).toMatchObject({ amount_cents: 18000 })
  })

  it('o acerto de saída do casal é rotulado', async () => {
    fakeServer({
      'GET /v1/finance/balance': balance(),
      'GET /v1/finance/settlements': [{ id: 's1', through_on: '2026-10-30', from_user: 'bia', to_user: 'me', amount_cents: 5000, kind: 'leave', created_by: 'bia', created_at: '' }],
    })
    renderWithQuery(<BalanceCard ctx={ctx} />)
    expect(await screen.findByText('Acerto na saída do casal')).toBeInTheDocument()
  })
})
