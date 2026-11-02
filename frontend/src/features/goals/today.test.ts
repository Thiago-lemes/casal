import { describe, expect, it } from 'vitest'
import type { Goal } from './api'
import { goalsForToday } from './today'

const base: Goal = {
  id: 'g', created_by: 'me', owner_id: 'me', scope: 'personal', visibility: 'private', kind: 'habit', title: 'Correr',
  status: 'active', due_on: null, version: 1, access: 'full', can_edit: true, created_at: '', updated_at: '',
  habit: { target_count: 3, period: 'week', weekdays: 0, remind_min: 540, remind_enabled: false },
  progress: { done: 1, total: 3, percent: 33, unit: 'count', marked_today: false },
}
const goal = (over: Partial<Goal>): Goal => ({ ...base, ...over })
const MONDAY = '2026-11-02'

describe('goalsForToday', () => {
  it('hábito ativo sem marcação de hoje entra; já marcado não', () => {
    const r = goalsForToday([goal({ id: 'a' }), goal({ id: 'b', progress: { ...base.progress, marked_today: true } })], 'me', MONDAY)
    expect(r.habits.map((g) => g.id)).toEqual(['a'])
  })

  it('fora dos dias combinados não entra', () => {
    const tuesdays = goal({ habit: { ...base.habit!, weekdays: 0b0000010 } })
    expect(goalsForToday([tuesdays], 'me', MONDAY).habits).toHaveLength(0)
    expect(goalsForToday([tuesdays], 'me', '2026-11-03').habits).toHaveLength(1)
  })

  it('meta do período já batida não insiste (hábito); marcar mais continua possível na tela da meta', () => {
    const done = goal({ progress: { ...base.progress, done: 3, percent: 100 } })
    expect(goalsForToday([done], 'me', MONDAY).habits).toHaveLength(0)
  })

  it('meta do casal com o período cumprido também não insiste', () => {
    const done = goal({ kind: 'shared', scope: 'household', owner_id: null, habit: undefined, progress: { ...base.progress, done: 3, percent: 100 } })
    expect(goalsForToday([done], 'me', MONDAY).shared).toHaveLength(0)
  })

  it('pausada, concluída e as do parceiro (view) ficam de fora', () => {
    const list = [goal({ id: 'p', status: 'paused' }), goal({ id: 'd', status: 'done' }), goal({ id: 'v', access: 'view', owner_id: 'bia' })]
    expect(goalsForToday(list, 'me', MONDAY).habits).toHaveLength(0)
  })

  it('meta do casal ativa sem marcação de hoje entra; hábito de outra pessoa nunca', () => {
    const shared = goal({ id: 's', kind: 'shared', scope: 'household', owner_id: null, visibility: 'household', habit: undefined })
    const others = goal({ id: 'o', owner_id: 'bia' })
    const r = goalsForToday([shared, others], 'me', MONDAY)
    expect(r.shared.map((g) => g.id)).toEqual(['s'])
    expect(r.habits).toHaveLength(0)
  })
})
