import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ctx, expense, fakeServer, renderWithQuery } from '../testkit'
import { ExpenseComposer } from './ExpenseComposer'

afterEach(() => vi.unstubAllGlobals())

async function open(props: Partial<React.ComponentProps<typeof ExpenseComposer>> = {}) {
  renderWithQuery(<ExpenseComposer ctx={ctx} open onOpenChange={() => {}} {...props} />)
  await screen.findByRole('option', { name: 'Mercado' })
}

describe('ExpenseComposer', () => {
  it('gasto do casal mostra quem pagou e a divisão; pessoal esconde os dois e mostra a visibilidade', async () => {
    fakeServer({})
    await open()
    expect(screen.getByText('Quem pagou')).toBeInTheDocument()
    expect(screen.getByText('Divisão')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('radio', { name: 'Pessoal' }))
    expect(screen.queryByText('Quem pagou')).not.toBeInTheDocument()
    expect(screen.queryByText('Divisão')).not.toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Só eu' })).toBeChecked()
  })

  it('só oferece categorias não arquivadas em gasto novo', async () => {
    fakeServer({})
    await open()
    expect(screen.queryByRole('option', { name: /Antiga/ })).not.toBeInTheDocument()
  })

  it('envia o valor em centavos inteiros, sem divisão (o servidor aplica a padrão)', async () => {
    const server = fakeServer({ 'POST /v1/finance/expenses': expense() })
    await open()
    await userEvent.type(screen.getByLabelText('Valor'), '1.234,56')
    await userEvent.click(screen.getByRole('button', { name: 'Registrar gasto' }))

    await waitFor(() => expect(server.sent('POST', '/v1/finance/expenses')).toHaveLength(1))
    const body = server.sent('POST', '/v1/finance/expenses')[0].body!
    expect(body).toMatchObject({ scope: 'household', amount_cents: 123456, category_id: 'c-groc', paid_by: 'me' })
    expect(body).not.toHaveProperty('split')
    expect(body).not.toHaveProperty('installments')
  })

  it('parcelar com o valor da parcela não envia amount_cents', async () => {
    const server = fakeServer({ 'POST /v1/finance/expenses': { expenses: [expense(), expense()] } })
    await open()
    await userEvent.type(screen.getByLabelText('Valor'), '100')
    await userEvent.click(screen.getByRole('checkbox', { name: 'Foi parcelado' }))
    await userEvent.click(screen.getByRole('radio', { name: 'O valor da parcela' }))
    await userEvent.clear(screen.getByLabelText('Número de parcelas'))
    await userEvent.type(screen.getByLabelText('Número de parcelas'), '3')
    expect(screen.getByText(/3 × .*100,00 = .*300,00/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Registrar gasto' }))

    await waitFor(() => expect(server.sent('POST', '/v1/finance/expenses')).toHaveLength(1))
    const body = server.sent('POST', '/v1/finance/expenses')[0].body!
    expect(body).not.toHaveProperty('amount_cents')
    expect(body.installments).toMatchObject({ count: 3, installment_amount_cents: 10000 })
  })

  it('parcelar informando o total envia amount_cents e a contagem', async () => {
    const server = fakeServer({ 'POST /v1/finance/expenses': { expenses: [] } })
    await open()
    await userEvent.type(screen.getByLabelText('Valor total'.replace('Valor total', 'Valor')), '300')
    await userEvent.click(screen.getByRole('checkbox', { name: 'Foi parcelado' }))
    await userEvent.click(screen.getByRole('button', { name: 'Registrar gasto' }))
    await waitFor(() => expect(server.sent('POST', '/v1/finance/expenses')).toHaveLength(1))
    const body = server.sent('POST', '/v1/finance/expenses')[0].body!
    expect(body.amount_cents).toBe(30000)
    expect(body.installments).toMatchObject({ count: 2 })
    expect((body.installments as Record<string, unknown>).installment_amount_cents).toBeUndefined()
  })

  it('valor inválido bloqueia o envio', async () => {
    fakeServer({})
    await open()
    await userEvent.type(screen.getByLabelText('Valor'), '12,345')
    expect(screen.getByText(/entre R\$ 0,01 e R\$ 10\.000\.000,00/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Registrar gasto' })).toBeDisabled()
  })

  it('"Personalizar" mostra o controle e envia pontos-base que somam 10000', async () => {
    const server = fakeServer({ 'POST /v1/finance/expenses': expense() })
    await open()
    await userEvent.type(screen.getByLabelText('Valor'), '50')
    await userEvent.click(screen.getByRole('radio', { name: 'Personalizar' }))
    expect(screen.getByLabelText('Parte de cada um')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Registrar gasto' }))
    await waitFor(() => expect(server.sent('POST', '/v1/finance/expenses')).toHaveLength(1))
    const split = server.sent('POST', '/v1/finance/expenses')[0].body!.split as Record<string, number>
    expect(Object.values(split).reduce((a, b) => a + b, 0)).toBe(10000)
  })

  it('conflito de versão: mostra a mensagem, mantém o que foi digitado e reenvia com a versão nova', async () => {
    let puts = 0
    const server = fakeServer({
      'PUT /v1/finance/expenses/e1': () => (++puts === 1 ? { status: 409, code: 'expense_conflict' } : expense({ version: 3 })),
      'GET /v1/finance/expenses/e1': expense({ version: 2, amount_cents: 99900, description: 'Mudou' }),
    })
    await open({ expense: expense() as never })
    await userEvent.clear(screen.getByLabelText('Descrição (opcional)'))
    await userEvent.type(screen.getByLabelText('Descrição (opcional)'), 'Minha edição')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))

    expect(await screen.findByText(/Outra pessoa mudou este gasto/)).toBeInTheDocument()
    expect(screen.getByLabelText('Descrição (opcional)')).toHaveValue('Minha edição')

    await userEvent.click(await screen.findByRole('button', { name: 'Aplicar de novo' }))
    await waitFor(() => expect(server.sent('PUT', '/v1/finance/expenses/e1')).toHaveLength(2))
    expect(server.sent('PUT', '/v1/finance/expenses/e1')[0].body).toMatchObject({ version: 1 })
    expect(server.sent('PUT', '/v1/finance/expenses/e1')[1].body).toMatchObject({ version: 2, description: 'Minha edição' })
  })
})
