import { describe, expect, it } from 'vitest'
import type { Goal } from '../api'
import { allowedScopes, buildInput, buildUpdate, emptyForm, formFromGoal, minToTime, parseTarget, timeToMin, validate } from './form'
import type { Form } from './form'

const form = (over: Partial<Form>): Form => ({ ...emptyForm, title: 'Meta', ...over })
const TODAY = '2026-11-02'

describe('escopos permitidos', () => {
  it('hábito só pessoal, meta do casal só do casal, o resto os dois', () => {
    expect(allowedScopes('habit')).toEqual(['personal'])
    expect(allowedScopes('shared')).toEqual(['household'])
    expect(allowedScopes('financial')).toEqual(['personal', 'household'])
    expect(allowedScopes('project')).toEqual(['personal', 'household'])
  })
})

describe('validate', () => {
  it('título de 1 a 120 caracteres', () => {
    expect(validate(form({ kind: 'project', title: '  ' }), TODAY).title).toBe('title')
    expect(validate(form({ kind: 'project', title: 'x'.repeat(121) }), TODAY).title).toBe('title')
    expect(validate(form({ kind: 'project' }), TODAY)).toEqual({})
  })

  it('prazo no passado é recusado, a menos que não tenha mudado', () => {
    expect(validate(form({ kind: 'project', dueOn: '2026-11-01' }), TODAY).dueOn).toBe('dueOnPast')
    expect(validate(form({ kind: 'project', dueOn: '2026-11-01' }), TODAY, { unchangedDue: true })).toEqual({})
    expect(validate(form({ kind: 'project', dueOn: TODAY }), TODAY)).toEqual({})
  })

  it('semanal de 1 a 7, mensal de 1 a 31', () => {
    expect(parseTarget('7', 'week')).toBe(7)
    expect(parseTarget('8', 'week')).toBeNull()
    expect(parseTarget('31', 'month')).toBe(31)
    expect(parseTarget('0', 'month')).toBeNull()
    expect(parseTarget('2,5', 'week')).toBeNull()
    expect(validate(form({ kind: 'habit', target: '8' }), TODAY).target).toBe('targetWeek')
    expect(validate(form({ kind: 'shared', period: 'month', target: '32' }), TODAY).target).toBe('targetMonth')
  })

  it('valor alvo em reais precisa ser positivo e caber no teto', () => {
    expect(validate(form({ kind: 'financial', amount: '' }), TODAY).amount).toBe('amount')
    expect(validate(form({ kind: 'financial', amount: '0' }), TODAY).amount).toBe('amount')
    expect(validate(form({ kind: 'financial', amount: '10.000.000,01' }), TODAY).amount).toBe('amount')
    expect(validate(form({ kind: 'financial', amount: '1.000,50' }), TODAY)).toEqual({})
  })

  it('economia do orçamento pede a categoria', () => {
    expect(validate(form({ kind: 'financial', amount: '100', source: 'budget_savings' }), TODAY).category).toBe('category')
    expect(validate(form({ kind: 'financial', amount: '100', source: 'budget_savings', categoryId: 'c1' }), TODAY)).toEqual({})
  })
})

describe('buildInput', () => {
  it('hábito: máscara de dias e lembrete no minuto do dia', () => {
    const body = buildInput(form({ kind: 'habit', scope: 'personal', target: '3', days: [1, 3, 5], remindOn: true, remindTime: '07:30' }))
    expect(body).toEqual({
      kind: 'habit', scope: 'personal', visibility: 'private', title: 'Meta',
      habit: { target_count: 3, period: 'week', weekdays: 0b10101, remind_enabled: true, remind_min: 450 },
    })
  })

  it('hábito sem lembrete não manda horário', () => {
    const body = buildInput(form({ kind: 'habit', scope: 'personal' }))
    expect(body.habit).toEqual({ target_count: 3, period: 'week', weekdays: 0, remind_enabled: false })
  })

  it('meta do casal não tem visibilidade e leva requires_both', () => {
    const body = buildInput(form({ kind: 'shared', scope: 'household', target: '1', period: 'month', requiresBoth: true }))
    expect(body.visibility).toBeUndefined()
    expect(body.shared).toEqual({ target_count: 1, period: 'month', requires_both: true })
  })

  it('financeira: valor em centavos inteiros, sem float', () => {
    const body = buildInput(form({ kind: 'financial', scope: 'personal', amount: '1.234,56', dueOn: '2027-01-10', notes: ' nota ' }))
    expect(body.financial).toEqual({ target_cents: 123456, source: 'contributions' })
    expect(body.due_on).toBe('2027-01-10')
    expect(body.notes).toBe('nota')
  })

  it('economia do orçamento leva a categoria', () => {
    const body = buildInput(form({ kind: 'financial', scope: 'household', amount: '500', source: 'budget_savings', categoryId: 'c1' }))
    expect(body.financial).toEqual({ target_cents: 50000, source: 'budget_savings', category_id: 'c1' })
  })

  it('projeto: ignora linhas vazias; tarefa só no projeto do casal', () => {
    const lines = ['  Pintar ', '', 'Comprar tinta']
    expect(buildInput(form({ kind: 'project', scope: 'household', steps: lines, stepTasks: true })).steps).toEqual([
      { title: 'Pintar', task: true }, { title: 'Comprar tinta', task: true },
    ])
    expect(buildInput(form({ kind: 'project', scope: 'personal', steps: lines, stepTasks: true })).steps).toEqual([
      { title: 'Pintar' }, { title: 'Comprar tinta' },
    ])
  })
})

const habit: Goal = {
  id: 'g1', created_by: 'me', owner_id: 'me', scope: 'personal', visibility: 'view', kind: 'habit', title: 'Correr', notes: 'n',
  status: 'active', due_on: null, version: 4, access: 'full', can_edit: true, created_at: '', updated_at: '',
  habit: { target_count: 2, period: 'week', weekdays: 0b0000101, remind_min: 540, remind_enabled: true },
  progress: { done: 0, total: 2, percent: 0, unit: 'count' },
}

describe('edição', () => {
  it('o formulário nasce da meta e volta ao corpo do PUT com a versão', () => {
    const f = formFromGoal(habit)
    expect(f).toMatchObject({ target: '2', days: [1, 3], remindOn: true, remindTime: '09:00', visibility: 'view' })
    expect(buildUpdate(habit, f)).toEqual({
      version: 4, title: 'Correr', notes: 'n', visibility: 'view',
      habit: { target_count: 2, weekdays: 0b101, remind_enabled: true, remind_min: 540 },
    })
  })

  it('meta do casal não manda visibilidade', () => {
    const shared: Goal = { ...habit, kind: 'shared', scope: 'household', owner_id: null, visibility: 'household', habit: undefined, shared: { target_count: 1, period: 'month', requires_both: false } }
    const body = buildUpdate(shared, formFromGoal(shared))
    expect(body.visibility).toBeUndefined()
    expect(body.shared).toEqual({ target_count: 1, requires_both: false })
  })
})

describe('horário', () => {
  it('ida e volta', () => {
    expect(timeToMin('09:00')).toBe(540)
    expect(minToTime(450)).toBe('07:30')
    expect(minToTime(timeToMin('23:59'))).toBe('23:59')
  })
})
