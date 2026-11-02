import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ComponentProps } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/shared/api/client'
import type { Block } from './api'
import { BlockEditor } from './BlockEditor'

const block = (over: Partial<Block> = {}): Block => ({
  id: 'b1',
  owner_id: 'me',
  owner_name: 'Ana',
  title: 'Trabalho',
  category: 'work',
  location: 'Escritório',
  visibility: 'busy',
  busy: false,
  recurrence: 'weekly',
  weekdays: [1, 2, 3],
  interval_weeks: 1,
  start_date: '2026-11-02',
  end_date: null,
  start: '09:00',
  end: '18:00',
  version: 5,
  ...over,
})

function setup(props: Partial<ComponentProps<typeof BlockEditor>> = {}) {
  const onSave = vi.fn()
  render(<BlockEditor open onOpenChange={() => {}} block={null} today="2026-11-04" isOwner readOnly={false} onSave={onSave} {...props} />)
  return { onSave }
}

describe('BlockEditor', () => {
  it('um bloco novo começa privado, semanal e no dia da semana de hoje', () => {
    setup()
    expect(screen.getByRole('radio', { name: /Só eu/ })).toBeChecked()
    // "Toda semana" aparece na repetição e na frequência; as duas começam marcadas
    expect(screen.getAllByRole('radio', { name: 'Toda semana', checked: true })).toHaveLength(2)
    expect(screen.getByRole('checkbox', { name: 'Qua' })).toBeChecked() // 2026-11-04 é quarta
  })

  it('não salva sem título e explica o problema', async () => {
    const { onSave } = setup()
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.getByText('Dê um título ao bloco.')).toBeInTheDocument()
  })

  it('avisa quando o fim não é depois do início', async () => {
    const { onSave } = setup()
    await userEvent.type(screen.getByLabelText('Título'), 'Reunião')
    const end = screen.getByLabelText('Fim')
    await userEvent.clear(end)
    await userEvent.type(end, '08:00')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.getByText('O fim precisa ser depois do início, no mesmo dia.')).toBeInTheDocument()
  })

  it('exige ao menos um dia da semana quando repete', async () => {
    setup()
    await userEvent.type(screen.getByLabelText('Título'), 'Treino')
    await userEvent.click(screen.getByRole('checkbox', { name: 'Qua' })) // desmarca o único
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))
    expect(screen.getByText('Escolha pelo menos um dia.')).toBeInTheDocument()
  })

  it('"só uma vez" esconde dias e frequência, e envia sem eles', async () => {
    const { onSave } = setup()
    await userEvent.type(screen.getByLabelText('Título'), 'Dentista')
    await userEvent.click(screen.getByRole('radio', { name: 'Só uma vez' }))
    expect(screen.queryByText('Dias da semana')).not.toBeInTheDocument()
    expect(screen.queryByText('Frequência')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))
    expect(onSave.mock.calls[0][0]).toMatchObject({ title: 'Dentista', recurrence: 'once', weekdays: [], interval_weeks: 1, end_date: '', visibility: 'private' })
  })

  it('ao editar, devolve a versão que a pessoa viu', async () => {
    const { onSave } = setup({ block: block() })
    await userEvent.type(screen.getByLabelText('Título'), '!')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))
    expect(onSave.mock.calls[0][1]).toBe(5)
    expect(onSave.mock.calls[0][0]).toMatchObject({ title: 'Trabalho!', weekdays: [1, 2, 3], visibility: 'busy' })
  })

  it('bloco do casal que não é meu: edita, mas não muda a visibilidade nem apaga', () => {
    setup({ block: block({ owner_id: 'other', visibility: 'household' }), isOwner: false, onDelete: vi.fn() })
    expect(screen.getByRole('radio', { name: /Do casal/ })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Apagar bloco' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Salvar' })).toBeInTheDocument()
  })

  it('só leitura: sem salvar', () => {
    setup({ block: block({ owner_id: 'other', visibility: 'view' }), isOwner: false, readOnly: true })
    expect(screen.queryByRole('button', { name: 'Salvar' })).not.toBeInTheDocument()
  })

  it('conflito: carrega a versão atual e passa a salvar em cima dela', async () => {
    const onLoadCurrent = vi.fn().mockResolvedValue(block({ title: 'Trabalho (remoto)', version: 6 }))
    const { onSave } = setup({ block: block(), error: new ApiError(409, 'routine_conflict'), onLoadCurrent })
    expect(screen.getByRole('alert')).toHaveTextContent('Alguém alterou esse bloco antes de você')
    await userEvent.click(screen.getByRole('button', { name: 'Ver a versão atual' }))
    expect(await screen.findByDisplayValue('Trabalho (remoto)')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))
    expect(onSave.mock.calls[0][1]).toBe(6)
  })
})
