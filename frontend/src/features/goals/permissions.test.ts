import { describe, expect, it } from 'vitest'
import type { Goal, Kind, Status } from './api'
import { goalActions } from './permissions'

const goal = (over: Partial<Goal> = {}): Goal => ({
  id: 'g1', created_by: 'me', owner_id: 'me', scope: 'personal', visibility: 'private', kind: 'habit', title: 'Correr',
  status: 'active', due_on: null, version: 1, access: 'full', can_edit: true, created_at: '', updated_at: '',
  progress: { done: 0, total: 3, percent: 0, unit: 'count' }, ...over,
})

const kinds: Kind[] = ['financial', 'habit', 'shared', 'project']
const statuses: Status[] = ['active', 'paused', 'done', 'archived']

describe('goalActions', () => {
  it.each(kinds.flatMap((k) => statuses.map((s) => [k, s] as const)))('sem can_edit (%s, %s) nenhuma ação', (kind, status) => {
    expect(Object.values(goalActions(goal({ kind, status, can_edit: false, access: 'view' }))).some(Boolean)).toBe(false)
  })

  it('concluída: só reabrir e apagar', () => {
    const a = goalActions(goal({ status: 'done' }))
    expect(Object.entries(a).filter(([, v]) => v).map(([k]) => k)).toEqual(['resume', 'delete'])
  })

  it('arquivada: só desarquivar e apagar', () => {
    const a = goalActions(goal({ status: 'archived' }))
    expect(Object.entries(a).filter(([, v]) => v).map(([k]) => k)).toEqual(['unarchive', 'delete'])
  })

  it('ativa: pausa; pausada: retoma; as duas concluem e arquivam', () => {
    const active = goalActions(goal())
    expect(active).toMatchObject({ edit: true, pause: true, resume: false, complete: true, archive: true })
    const paused = goalActions(goal({ status: 'paused' }))
    expect(paused).toMatchObject({ pause: false, resume: true, complete: true })
  })

  it('cada tipo oferece a sua ação de registro', () => {
    expect(goalActions(goal({ kind: 'habit' })).mark).toBe(true)
    expect(goalActions(goal({ kind: 'shared' })).mark).toBe(true)
    expect(goalActions(goal({ kind: 'project' }))).toMatchObject({ addStep: true, mark: false, contribute: false })
    expect(goalActions(goal({ kind: 'financial', financial: { target_cents: 1, source: 'contributions', baseline_on: '' } })).contribute).toBe(true)
  })

  it('economia do orçamento não tem aporte', () => {
    const a = goalActions(goal({ kind: 'financial', scope: 'household', financial: { target_cents: 1, source: 'budget_savings', baseline_on: '' } }))
    expect(a.contribute).toBe(false)
    expect(a.edit).toBe(true)
  })
})
