import { describe, expect, it } from 'vitest'
import type { Task } from './api'
import { sortDone, sortOpen, taskActions, toAssignment } from './permissions'

const ME = 'me'
const OTHER = 'other'

const task = (over: Partial<Task>): Task => ({
  id: 't',
  created_by: ME,
  created_by_name: 'Ana',
  assignee_id: ME,
  title: 'x',
  status: 'open',
  created_at: '2026-11-01T10:00:00Z',
  ...over,
})

describe('taskActions', () => {
  it('tarefa minha, criada por mim: conclui e gerencia, mas não recusa a própria', () => {
    expect(taskActions(task({}), ME)).toEqual({ complete: true, decline: false, assume: false, reassign: true, cancel: true })
  })

  it('tarefa que o parceiro criou para mim: conclui e recusa; não gerencia', () => {
    const a = taskActions(task({ created_by: OTHER }), ME)
    expect(a).toEqual({ complete: true, decline: true, assume: false, reassign: false, cancel: false })
  })

  it('tarefa que criei para o parceiro: concluo (sou o criador), não recuso, posso assumir', () => {
    const a = taskActions(task({ assignee_id: OTHER }), ME)
    expect(a).toEqual({ complete: true, decline: false, assume: true, reassign: true, cancel: true })
  })

  it('tarefa que o parceiro criou para ele: não faço nada', () => {
    const a = taskActions(task({ created_by: OTHER, assignee_id: OTHER }), ME)
    expect(a).toEqual({ complete: false, decline: false, assume: false, reassign: false, cancel: false })
  })

  it('tarefa do casal: qualquer um conclui; só o criador gerencia; ninguém recusa', () => {
    const shared = task({ assignee_id: null })
    expect(taskActions(shared, ME)).toEqual({ complete: true, decline: false, assume: true, reassign: true, cancel: true })
    expect(taskActions({ ...shared, created_by: OTHER }, ME)).toEqual({
      complete: true,
      decline: false,
      assume: false,
      reassign: false,
      cancel: false,
    })
  })

  it('recusada: o criador decide (assumir, passar, cancelar); ninguém conclui', () => {
    const declined = task({ status: 'declined', assignee_id: OTHER })
    expect(taskActions(declined, ME)).toEqual({ complete: false, decline: false, assume: true, reassign: true, cancel: true })
    expect(taskActions({ ...declined, created_by: OTHER, assignee_id: ME }, ME)).toEqual({
      complete: false,
      decline: false,
      assume: false,
      reassign: false,
      cancel: false,
    })
  })

  it('concluída ou cancelada: sem nenhuma ação', () => {
    for (const status of ['done', 'canceled'] as const) {
      expect(Object.values(taskActions(task({ status }), ME))).not.toContain(true)
    }
  })
})

describe('toAssignment', () => {
  it('eu, parceiro e do casal', () => {
    expect(toAssignment('me', ME, OTHER)).toEqual({ assignee_id: ME, shared: false })
    expect(toAssignment('partner', ME, OTHER)).toEqual({ assignee_id: OTHER, shared: false })
    expect(toAssignment('shared', ME, OTHER)).toEqual({ assignee_id: '', shared: true })
  })

  it('"parceiro" sem parceiro cai em mim, nunca em branco', () => {
    expect(toAssignment('partner', ME)).toEqual({ assignee_id: ME, shared: false })
  })
})

describe('ordenação', () => {
  it('abertas: prazo mais próximo antes; sem prazo por último, as mais antigas antes', () => {
    const sorted = sortOpen([
      task({ id: 'sem-novo', created_at: '2026-11-03T00:00:00Z' }),
      task({ id: 'tarde', due_at: '2026-11-09T00:00:00Z' }),
      task({ id: 'sem-velho', created_at: '2026-11-01T00:00:00Z' }),
      task({ id: 'cedo', due_at: '2026-11-02T00:00:00Z' }),
    ])
    expect(sorted.map((t) => t.id)).toEqual(['cedo', 'tarde', 'sem-velho', 'sem-novo'])
  })

  it('concluídas: mais recentes primeiro', () => {
    const sorted = sortDone([
      task({ id: 'a', completed_at: '2026-11-01T10:00:00Z' }),
      task({ id: 'b', completed_at: '2026-11-02T10:00:00Z' }),
    ])
    expect(sorted.map((t) => t.id)).toEqual(['b', 'a'])
  })
})
