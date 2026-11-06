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

  it('manda só o campo que mudou: o fuso escolhido', async () => {
    const update = idle()
    render(<ProfilePage me={me} update={update} />)
    await userEvent.selectOptions(screen.getByLabelText('Fuso horário'), 'America/Manaus')
    await userEvent.click(screen.getAllByRole('button', { name: 'Salvar' })[1])
    expect(update.mutate).toHaveBeenCalledWith({ timezone: 'America/Manaus' })
  })

  it('o rótulo traz o deslocamento', () => {
    render(<ProfilePage me={me} update={idle()} />)
    expect(screen.getByRole('option', { name: /^Brasília \(GMT−3\)$/ })).toBeInTheDocument()
  })

  it('um fuso que não está na lista continua aparecendo como opção', () => {
    render(<ProfilePage me={{ ...me, timezone: 'Africa/Maputo' }} update={idle()} />)
    expect(screen.getByRole('option', { name: /^Africa\/Maputo/ })).toBeInTheDocument()
  })

  it('"Outro fuso" aceita qualquer nome válido e recusa o inválido', async () => {
    const update = idle()
    render(<ProfilePage me={me} update={update} />)
    await userEvent.selectOptions(screen.getByLabelText('Fuso horário'), 'Outro fuso…')
    const field = screen.getByLabelText('Nome do fuso')
    await userEvent.type(field, 'Mars/Olympus')
    expect(screen.getByRole('alert')).toHaveTextContent('Não conheço esse fuso')
    await userEvent.clear(field)
    await userEvent.type(field, 'America/Lima')
    await userEvent.click(screen.getAllByRole('button', { name: 'Salvar' })[1])
    expect(update.mutate).toHaveBeenCalledWith({ timezone: 'America/Lima' })
  })

  it('apagar ou estragar o nome em "Outro fuso" desabilita o salvar, sem guardar o último válido', async () => {
    render(<ProfilePage me={me} update={idle()} />)
    await userEvent.selectOptions(screen.getByLabelText('Fuso horário'), 'Outro fuso…')
    const field = screen.getByLabelText('Nome do fuso')
    await userEvent.type(field, 'America/Lima')
    const save = screen.getAllByRole('button', { name: 'Salvar' })[1]
    expect(save).toBeEnabled()
    await userEvent.type(field, 'x')
    expect(save).toBeDisabled()
    await userEvent.clear(field)
    expect(save).toBeDisabled()
  })

  it('manda o nome do banco mesmo se a pessoa digitou em minúsculas', async () => {
    const update = idle()
    render(<ProfilePage me={me} update={update} />)
    await userEvent.selectOptions(screen.getByLabelText('Fuso horário'), 'Outro fuso…')
    await userEvent.type(screen.getByLabelText('Nome do fuso'), 'america/lima')
    await userEvent.click(screen.getAllByRole('button', { name: 'Salvar' })[1])
    expect(update.mutate).toHaveBeenCalledWith({ timezone: 'America/Lima' })
  })

  it('um fuso do Brasil fora da lista curta aparece com o nome da lista', () => {
    render(<ProfilePage me={{ ...me, timezone: 'America/Recife' }} update={idle()} />)
    expect(screen.getByLabelText('Fuso horário')).toHaveValue('America/Sao_Paulo')
    expect(screen.queryByRole('option', { name: /Recife/ })).not.toBeInTheDocument()
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
