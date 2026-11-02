import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { DeclineDialog } from './DeclineDialog'

function setup() {
  const onSubmit = vi.fn()
  render(<DeclineDialog open onOpenChange={() => {}} onSubmit={onSubmit} />)
  return { onSubmit, button: () => screen.getByRole('button', { name: 'Recusar tarefa' }) }
}

describe('DeclineDialog', () => {
  it('o botão só habilita com um motivo', async () => {
    const { button } = setup()
    expect(button()).toBeDisabled()

    await userEvent.type(screen.getByLabelText('Motivo'), '   ')
    expect(button()).toBeDisabled() // só espaços não vale

    await userEvent.type(screen.getByLabelText('Motivo'), 'Viajo')
    expect(button()).toBeEnabled()
  })

  it('envia o texto sem espaços nas pontas', async () => {
    const { button, onSubmit } = setup()
    await userEvent.type(screen.getByLabelText('Motivo'), '  Viajo nesse dia  ')
    await userEvent.click(button())
    expect(onSubmit).toHaveBeenCalledWith('Viajo nesse dia')
  })

  it('mostra o contador de caracteres', async () => {
    setup()
    await userEvent.type(screen.getByLabelText('Motivo'), 'abc')
    expect(screen.getByText('3/500')).toBeInTheDocument()
  })
})
