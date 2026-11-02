import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ProfilePage } from './ProfilePage'

const me = { name: 'Ana', email: 'ana@exemplo.com', timezone: 'America/Sao_Paulo' }
const idle = () => ({ mutate: vi.fn(), isPending: false, error: null, isSuccess: false })

describe('ProfilePage', () => {
  it('só habilita salvar quando algo mudou', async () => {
    render(<ProfilePage me={me} update={idle()} />)
    const [saveName, saveCity] = screen.getAllByRole('button', { name: 'Salvar' })
    expect(saveName).toBeDisabled()
    expect(saveCity).toBeDisabled()

    await userEvent.type(screen.getByLabelText('Nome'), ' Maria')
    expect(saveName).toBeEnabled()
  })

  it('manda só o campo que mudou: o fuso da cidade escolhida', async () => {
    const update = idle()
    render(<ProfilePage me={me} update={update} />)
    await userEvent.selectOptions(screen.getByLabelText('Cidade'), 'America/Manaus')
    await userEvent.click(screen.getAllByRole('button', { name: 'Salvar' })[1])
    expect(update.mutate).toHaveBeenCalledWith({ timezone: 'America/Manaus' })
  })

  it('um fuso que não está na lista continua aparecendo como opção', () => {
    render(<ProfilePage me={{ ...me, timezone: 'Africa/Maputo' }} update={idle()} />)
    expect(screen.getByRole('option', { name: 'Africa/Maputo' })).toBeInTheDocument()
  })

  it('sair do casal pede confirmação e explica as consequências', async () => {
    const leave = idle()
    render(<ProfilePage me={me} update={idle()} leave={leave} />)
    await userEvent.click(screen.getByRole('button', { name: 'Sair do casal' }))
    expect(screen.getByText('Isso não tem como desfazer.')).toBeInTheDocument()
    expect(leave.mutate).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: 'Sim, sair do casal' }))
    expect(leave.mutate).toHaveBeenCalledOnce()
  })

  it('sem casal, não oferece sair', () => {
    render(<ProfilePage me={me} update={idle()} />)
    expect(screen.queryByRole('button', { name: 'Sair do casal' })).not.toBeInTheDocument()
  })
})
