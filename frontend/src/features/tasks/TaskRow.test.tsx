import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Task } from './api'
import { TaskRow } from './TaskRow'

const me = { id: 'me', name: 'Ana', timezone: 'America/Sao_Paulo' }
const people = [me, { id: 'other', name: 'Bia' }]

const task = (over: Partial<Task>): Task => ({
  id: 't1',
  created_by: 'me',
  created_by_name: 'Ana',
  assignee_id: 'me',
  title: 'Lavar o banheiro',
  status: 'open',
  created_at: '2026-11-01T10:00:00Z',
  ...over,
})

function setup(t: Task) {
  const handlers = {
    onComplete: vi.fn(),
    onDecline: vi.fn(),
    onAssume: vi.fn(),
    onReassign: vi.fn(),
    onCancel: vi.fn(),
  }
  render(
    <ul>
      <TaskRow task={t} me={me} people={people} {...handlers} />
    </ul>,
  )
  return handlers
}

const has = (name: RegExp | string) => screen.queryByRole('button', { name }) !== null

describe('TaskRow', () => {
  it('tarefa que criei para mim: conclui e gerencia, mas não recusa', () => {
    setup(task({}))
    expect(has(/Concluir tarefa/)).toBe(true)
    expect(has('Recusar')).toBe(false)
    expect(has('Passar para outra pessoa')).toBe(true)
    expect(has('Cancelar tarefa')).toBe(true)
  })

  it('tarefa que a outra pessoa criou para mim: conclui e recusa; sem gerenciar', () => {
    setup(task({ created_by: 'other', created_by_name: 'Bia' }))
    expect(has(/Concluir tarefa/)).toBe(true)
    expect(has('Recusar')).toBe(true)
    expect(has('Passar para outra pessoa')).toBe(false)
    expect(has('Cancelar tarefa')).toBe(false)
    expect(screen.getByText('Criada por Bia')).toBeInTheDocument()
  })

  it('tarefa da outra pessoa para ela: o botão de concluir existe mas está desabilitado', () => {
    setup(task({ created_by: 'other', assignee_id: 'other' }))
    expect(screen.getByRole('button', { name: /Concluir tarefa/ })).toBeDisabled()
    expect(screen.queryByText('Recusar')).not.toBeInTheDocument()
  })

  it('tarefa do casal aparece como "Do casal"', () => {
    setup(task({ assignee_id: null }))
    expect(screen.getByText('Do casal')).toBeInTheDocument()
  })

  it('recusada, vista pelo criador: mostra o motivo e as decisões', () => {
    setup(task({ status: 'declined', assignee_id: 'other', decline_reason: 'Viajo nesse dia' }))
    expect(screen.getByText('Viajo nesse dia')).toBeInTheDocument()
    expect(screen.getByText('Bia recusou')).toBeInTheDocument()
    expect(has('Assumir')).toBe(true)
    expect(has('Passar para outra pessoa')).toBe(true)
    expect(has('Cancelar tarefa')).toBe(true)
    expect(screen.getByRole('button', { name: /Concluir tarefa/ })).toBeDisabled()
  })

  it('recusada, vista por quem recusou (não é o criador): só leitura', () => {
    setup(task({ status: 'declined', created_by: 'other', assignee_id: 'me', decline_reason: 'Não consigo' }))
    expect(has('Assumir') || has('Passar para outra pessoa') || has('Cancelar tarefa')).toBe(false)
  })

  it('concluída: mostra quem concluiu e nenhuma ação', () => {
    setup(task({ status: 'done', completed_by: 'other', completed_at: '2026-11-01T15:00:00Z' }))
    expect(screen.getByText(/Bia concluiu em/)).toBeInTheDocument()
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })

  it('dispara a ação certa ao tocar', async () => {
    const h = setup(task({ created_by: 'other', created_by_name: 'Bia' }))
    await userEvent.click(screen.getByRole('button', { name: /Concluir tarefa/ }))
    await userEvent.click(screen.getByText('Recusar'))
    expect(h.onComplete).toHaveBeenCalledOnce()
    expect(h.onDecline).toHaveBeenCalledOnce()
  })
})
