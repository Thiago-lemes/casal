import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { BIA, ME, expense, fakeServer, renderWithQuery } from './testkit'
import { FinancePage } from './FinancePage'

afterEach(() => vi.unstubAllGlobals())

const page = () => renderWithQuery(<FinancePage me={ME} people={[ME, BIA]} />)

describe('FinancePage', () => {
  it('módulo desligado: convite para ligar, sem tabs e sem erro', async () => {
    const server = fakeServer({
      'GET /v1/modules': [{ name: 'finance', enabled: false }],
      'PUT /v1/modules/finance': { name: 'finance', enabled: true },
    })
    page()
    expect(await screen.findByText('Ligue Finanças para começar')).toBeInTheDocument()
    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Ligar Finanças' }))
    await waitFor(() => expect(server.sent('PUT', '/v1/modules/finance')).toHaveLength(1))
    expect(server.sent('PUT', '/v1/modules/finance')[0].body).toEqual({ enabled: true })
  })

  it('módulo ligado: abas e estado vazio dizem o próximo passo', async () => {
    fakeServer({
      'GET /v1/modules': [{ name: 'finance', enabled: true }],
      'GET /v1/finance/expenses': { expenses: [] },
    })
    page()
    expect(await screen.findByRole('tab', { name: 'Gastos' })).toBeInTheDocument()
    expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Gastos', 'Saldo', 'Contas', 'Resumo'])
    expect(await screen.findByText('Nenhum gasto neste mês')).toBeInTheDocument()
    expect(screen.getByText(/Registre o primeiro gasto do mês/)).toBeInTheDocument()
  })

  it('403 module_disabled vira o convite (e recarrega os módulos), não uma falha genérica', async () => {
    let modules = 0
    fakeServer({
      'GET /v1/modules': () => [{ name: 'finance', enabled: ++modules === 1 }],
      'GET /v1/finance/expenses': { status: 403, code: 'module_disabled' },
    })
    page()
    expect(await screen.findByText('Ligue Finanças para começar')).toBeInTheDocument()
  })

  it('lista os gastos com selos e abre o detalhe só com as ações permitidas', async () => {
    fakeServer({
      'GET /v1/modules': [{ name: 'finance', enabled: true }],
      'GET /v1/finance/expenses': {
        expenses: [
          expense({ id: 'a', description: 'Feira' }),
          expense({ id: 'b', description: 'Aluguel', settled: true, can_edit: false, source: 'recurring' }),
          expense({ id: 'c', description: 'Sofá', installment: { group_id: 'g1', no: 3, total: 10 } }),
          expense({ id: 'd', description: 'Presente', scope: 'personal', visibility: 'view', created_by: 'bia', paid_by: 'bia', can_edit: false }),
        ],
      },
    })
    page()
    expect(await screen.findByText('Parcela 3/10')).toBeInTheDocument()
    expect(screen.getByText('Acertado')).toBeInTheDocument()
    expect(screen.getByText('Fixo')).toBeInTheDocument()
    expect(screen.getByText('Compartilhado pelo parceiro')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Ver gasto: Presente' }))
    let dialog = await screen.findByRole('dialog')
    expect(within(dialog).queryByRole('button', { name: 'Editar' })).not.toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: 'Apagar' })).not.toBeInTheDocument()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Fechar' }))

    await userEvent.click(screen.getByRole('button', { name: 'Ver gasto: Feira' }))
    dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('button', { name: 'Editar' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Apagar' })).toBeInTheDocument()
  })

  it('apagar um gasto já acertado mostra como corrigir (gasto de ajuste)', async () => {
    fakeServer({
      'GET /v1/modules': [{ name: 'finance', enabled: true }],
      'GET /v1/finance/expenses': { expenses: [expense({ id: 'a', description: 'Feira' })] },
      'DELETE /v1/finance/expenses/a': { status: 409, code: 'expense_settled' },
    })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Ver gasto: Feira' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Apagar' }))
    await userEvent.click(within(dialog).getByRole('button', { name: 'Apagar' }))
    expect(await within(dialog).findByText(/registre um gasto de ajuste/)).toBeInTheDocument()
  })
})
