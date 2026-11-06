import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { FirstSteps } from './FirstSteps'
import { buildSteps } from './progress'
import type { StepInput } from './progress'

const base: StepInput = { hasPartner: false, hasReminder: false, push: 'off', hasModule: false, cityChecked: false }

function setup(input: Partial<StepInput> = {}, extra: Partial<Parameters<typeof FirstSteps>[0]> = {}) {
  const props = { onDismiss: vi.fn(), onNewReminder: vi.fn(), onEnablePush: vi.fn() }
  render(
    <MemoryRouter>
      <Routes>
        <Route path="/" element={<FirstSteps steps={buildSteps({ ...base, ...input })} {...props} {...extra} />} />
        <Route path="/profile" element={<p>Tela do perfil</p>} />
        <Route path="/home" element={<p>Tela da casa</p>} />
      </Routes>
    </MemoryRouter>,
  )
  return props
}

describe('FirstSteps', () => {
  it('mostra o progresso e uma ação por passo pendente', () => {
    setup({ hasPartner: true })
    expect(screen.getByText('1 de 5')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Abrir o perfil' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Novo lembrete' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Ativar avisos' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Gerar convite' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('Feito')).toBeInTheDocument()
  })

  it('o botão do lembrete abre o compositor e o dos avisos pede a permissão', async () => {
    const props = setup()
    await userEvent.click(screen.getByRole('button', { name: 'Novo lembrete' }))
    await userEvent.click(screen.getByRole('button', { name: 'Ativar avisos' }))
    expect(props.onNewReminder).toHaveBeenCalledOnce()
    expect(props.onEnablePush).toHaveBeenCalledOnce()
  })

  it('leva ao perfil e à casa sem recarregar a página', async () => {
    setup()
    await userEvent.click(screen.getByRole('button', { name: 'Abrir o perfil' }))
    expect(await screen.findByText('Tela do perfil')).toBeInTheDocument()
  })

  it('dispensa', async () => {
    const props = setup()
    await userEvent.click(screen.getByRole('button', { name: 'Dispensar os primeiros passos' }))
    expect(props.onDismiss).toHaveBeenCalledOnce()
  })

  it('no iPhone fora da tela inicial explica a instalação em vez de oferecer o botão', () => {
    setup({ push: 'install' }, { pushNeedsInstall: true })
    expect(screen.queryByRole('button', { name: 'Ativar avisos' })).not.toBeInTheDocument()
    expect(screen.getByText(/só funcionam com o app na tela inicial/)).toBeInTheDocument()
    expect(screen.getByText(/Adicionar à Tela de Início/)).toBeInTheDocument()
  })

  it('mostra o erro de ativar os avisos', () => {
    setup({}, { pushError: 'Os avisos foram bloqueados.' })
    expect(screen.getByRole('alert')).toHaveTextContent('Os avisos foram bloqueados.')
  })
})
