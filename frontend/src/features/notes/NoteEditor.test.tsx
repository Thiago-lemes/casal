import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ComponentProps } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/shared/api/client'
import type { Note } from './api'
import { NoteEditor } from './NoteEditor'

const note = (over: Partial<Note> = {}): Note => ({
  id: 'n1',
  owner_id: 'me',
  owner_name: 'Ana',
  title: 'Mercado',
  kind: 'checklist',
  body: '',
  items: [{ text: 'Leite', done: false }],
  tags: ['casa'],
  visibility: 'private',
  pinned: false,
  version: 3,
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-01T10:00:00Z',
  ...over,
})

function setup(props: Partial<ComponentProps<typeof NoteEditor>> = {}) {
  const onSave = vi.fn()
  render(<NoteEditor open onOpenChange={() => {}} note={null} isOwner readOnly={false} onSave={onSave} {...props} />)
  return { onSave }
}

describe('NoteEditor', () => {
  it('uma anotação nova começa privada e só salva com algum conteúdo', async () => {
    const { onSave } = setup()
    expect(screen.getByRole('radio', { name: /Só eu/ })).toBeChecked()
    const save = screen.getByRole('button', { name: 'Salvar' })
    expect(save).toBeDisabled()

    await userEvent.type(screen.getByLabelText('Título'), 'Ideias')
    expect(save).toBeEnabled()
    await userEvent.click(save)
    expect(onSave).toHaveBeenCalledWith(
      { title: 'Ideias', kind: 'text', body: '', items: [], tags: [], visibility: 'private', pinned: false },
      undefined,
    )
  })

  it('numa lista, apara os itens, descarta linhas em branco e envia as etiquetas separadas', async () => {
    const { onSave } = setup()
    await userEvent.click(screen.getByRole('radio', { name: 'Lista' }))
    await userEvent.type(screen.getByLabelText('Texto do item 1'), '  Leite ')
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar item' }))
    await userEvent.type(screen.getByLabelText('Etiquetas'), 'casa, mercado,, casa')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))

    expect(onSave.mock.calls[0][0]).toMatchObject({
      kind: 'checklist',
      items: [{ text: 'Leite', done: false }],
      tags: ['casa', 'mercado'],
    })
  })

  it('ao editar, devolve a versão que a pessoa viu', async () => {
    const { onSave } = setup({ note: note() })
    await userEvent.type(screen.getByLabelText('Título'), '!')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))
    expect(onSave.mock.calls[0][1]).toBe(3)
  })

  it('anotação do parceiro só de leitura: nada editável e sem botão de salvar', () => {
    setup({ note: note({ owner_id: 'other', visibility: 'view' }), isOwner: false, readOnly: true })
    expect(screen.queryByRole('button', { name: 'Salvar' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('Título')).toHaveAttribute('readonly')
    expect(screen.getByRole('radio', { name: /A outra pessoa lê/ })).toBeDisabled()
  })

  it('em anotação do casal que não é minha, não muda a visibilidade nem apaga', () => {
    setup({ note: note({ owner_id: 'other', visibility: 'household' }), isOwner: false, onDelete: vi.fn() })
    expect(screen.getByRole('radio', { name: /Do casal/ })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Apagar anotação' })).not.toBeInTheDocument()
  })

  it('apagar pede uma segunda confirmação', async () => {
    const onDelete = vi.fn()
    setup({ note: note(), onDelete })
    await userEvent.click(screen.getByRole('button', { name: 'Apagar anotação' }))
    expect(onDelete).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: 'Confirmar: apagar' }))
    expect(onDelete).toHaveBeenCalledOnce()
  })

  it('conflito: explica e carrega a versão atual no rascunho', async () => {
    const onLoadCurrent = vi.fn().mockResolvedValue(note({ title: 'Mercado (do parceiro)', version: 4 }))
    const { onSave } = setup({ note: note(), error: new ApiError(409, 'note_conflict'), onLoadCurrent })

    expect(screen.getByRole('alert')).toHaveTextContent('Alguém alterou essa anotação antes de você')
    await userEvent.click(screen.getByRole('button', { name: 'Ver a versão atual' }))
    expect(await screen.findByDisplayValue('Mercado (do parceiro)')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Salvar' }))
    expect(onSave.mock.calls[0][1]).toBe(4) // agora salva em cima da versão nova
  })
})
