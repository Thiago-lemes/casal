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

  it('o cartão é opcional: sem escolher, não envia card_id', async () => {
    const server = fakeServer({ 'POST /v1/finance/expenses': expense(), 'GET /v1/finance/cards': [{ id: 'k1', name: 'Nubank', last4: '4321', archived: false }] })
    await open()
    await screen.findByRole('option', { name: 'Nubank · 4321' })
    await userEvent.type(screen.getByLabelText('Valor'), '10')
    await userEvent.click(screen.getByRole('button', { name: 'Registrar gasto' }))
    await waitFor(() => expect(server.sent('POST', '/v1/finance/expenses')).toHaveLength(1))
    expect(server.sent('POST', '/v1/finance/expenses')[0].body).not.toHaveProperty('card_id')
  })

  it('escolher um cartão envia card_id; cadastrar na hora já deixa o novo escolhido', async () => {
    const server = fakeServer({
      'POST /v1/finance/expenses': expense(),
      'GET /v1/finance/cards': [{ id: 'k1', name: 'Nubank', archived: false }],
      'POST /v1/finance/cards': { id: 'k2', name: 'Itaú', archived: false },
    })
    await open()
    await screen.findByRole('option', { name: 'Nubank' })
    await userEvent.type(screen.getByLabelText('Valor'), '10')
    await userEvent.selectOptions(screen.getByLabelText('Cartão'), 'k1')
    await userEvent.click(screen.getByRole('button', { name: 'Registrar gasto' }))
    await waitFor(() => expect(server.sent('POST', '/v1/finance/expenses')).toHaveLength(1))
    expect(server.sent('POST', '/v1/finance/expenses')[0].body).toMatchObject({ card_id: 'k1' })

    await userEvent.selectOptions(screen.getByLabelText('Cartão'), '__new__')
    await userEvent.type(screen.getByLabelText('Nome do cartão'), 'Itaú')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar cartão' }))
    await waitFor(() => expect(server.sent('POST', '/v1/finance/cards')).toHaveLength(1))
    expect(server.sent('POST', '/v1/finance/cards')[0].body).toEqual({ name: 'Itaú' })
  })

  it('quando o parceiro pagou, o campo de cartão some (cartão é de quem pagou)', async () => {
    fakeServer({})
    await open()
    expect(screen.getByLabelText('Cartão')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('radio', { name: ctx.partner!.name }))
    expect(screen.queryByLabelText('Cartão')).not.toBeInTheDocument()
  })

  it('editar mantém o cartão do gasto e o envia de volta', async () => {
    const server = fakeServer({
      'PUT /v1/finance/expenses/e1': expense({ card_id: 'k1' }),
      'GET /v1/finance/cards': [{ id: 'k1', name: 'Nubank', archived: false }],
    })
    await open({ expense: expense({ card_id: 'k1' }) as never })
    await screen.findByRole('option', { name: 'Nubank' })
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))
    await waitFor(() => expect(server.sent('PUT', '/v1/finance/expenses/e1')).toHaveLength(1))
    expect(server.sent('PUT', '/v1/finance/expenses/e1')[0].body).toMatchObject({ card_id: 'k1' })
  })
})
