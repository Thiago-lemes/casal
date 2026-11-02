import { describe, expect, it } from 'vitest'
import type { Task } from './api'
import { tasksForToday } from './today'

const now = new Date(2026, 10, 1, 15, 0)
const at = (day: number, h = 12) => new Date(2026, 10, day, h).toISOString()

const task = (id: string, over: Partial<Task>): Task => ({
  id,
  created_by: 'other',
  created_by_name: 'Bia',
  assignee_id: 'me',
  title: id,
  status: 'open',
  repeat: 'none',
  rotation: false,
  created_at: '2026-10-30T10:00:00Z',
  ...over,
})

describe('tasksForToday', () => {
  it('traz as que vencem hoje, as atrasadas e as sem prazo; deixa as de outros dias', () => {
    const out = tasksForToday(
      [
        task('hoje', { due_at: at(1, 20) }),
        task('atrasada', { due_at: at(30 - 31, 9) }),
        task('sem-prazo', {}),
        task('amanha', { due_at: at(2, 9) }),
      ],
      'me',
      now,
    )
    expect(out.map((t) => t.id)).toEqual(['atrasada', 'hoje', 'sem-prazo'])
  })

  it('só as minhas ou do casal; nunca as da outra pessoa', () => {
    const out = tasksForToday([task('minha', {}), task('do-casal', { assignee_id: null }), task('dela', { assignee_id: 'other' })], 'me', now)
    expect(out.map((t) => t.id).sort()).toEqual(['do-casal', 'minha'])
  })

  it('ignora o que não está aberto', () => {
    expect(tasksForToday([task('feita', { status: 'done' })], 'me', now)).toEqual([])
  })
})
