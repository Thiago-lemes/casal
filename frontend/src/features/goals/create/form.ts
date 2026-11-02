import { centsToInput, MAX_CENTS, parseMoney } from '@/shared/lib/money'
import type { Goal, GoalInput, GoalUpdate, Kind, Period } from '../api'
import { daysToMask, maskToDays } from '../weekdays'

type Scope = 'household' | 'personal'

/** O estado do formulário de criação e de edição; vira o corpo da API só no envio. */
export type Form = {
  kind: Kind | null
  scope: Scope | null
  visibility: 'private' | 'view'
  title: string
  notes: string
  dueOn: string
  /** hábito e meta do casal: quantas vezes por período */
  target: string
  period: Period
  days: number[]
  remindOn: boolean
  remindTime: string
  requiresBoth: boolean
  /** meta financeira */
  amount: string
  source: 'contributions' | 'budget_savings'
  categoryId: string
  /** projeto: um título por linha; `stepTasks` cria a tarefa de cada etapa (só projeto do casal) */
  steps: string[]
  stepTasks: boolean
}

export const emptyForm: Form = {
  kind: null, scope: null, visibility: 'private', title: '', notes: '', dueOn: '',
  target: '3', period: 'week', days: [], remindOn: false, remindTime: '09:00', requiresBoth: false,
  amount: '', source: 'contributions', categoryId: '', steps: [''], stepTasks: false,
}

/** Hábito é só pessoal (só o dono marca); meta do casal é só do casal; os outros tipos aceitam os dois. */
export function allowedScopes(kind: Kind): Scope[] {
  if (kind === 'habit') return ['personal']
  if (kind === 'shared') return ['household']
  return ['personal', 'household']
}

export const MAX_STEPS = 200

export type FieldErrors = Partial<Record<'title' | 'notes' | 'dueOn' | 'target' | 'amount' | 'category' | 'steps', string>>

export const timeToMin = (time: string): number => {
  const [h, m] = time.split(':').map(Number)
  return (Number.isFinite(h) ? h : 0) * 60 + (Number.isFinite(m) ? m : 0)
}

export const minToTime = (min: number): string => `${String(Math.floor(min / 60)).padStart(2, '0')}:${String(min % 60).padStart(2, '0')}`

/** Quantas vezes por período: semanal de 1 a 7, mensal de 1 a 31. */
export const maxTarget = (period: Period) => (period === 'week' ? 7 : 31)

export function parseTarget(text: string, period: Period): number | null {
  if (!/^\d+$/.test(text.trim())) return null
  const n = Number(text)
  return n >= 1 && n <= maxTarget(period) ? n : null
}

const titles = (f: Form) => f.steps.map((s) => s.trim()).filter(Boolean)

/**
 * Confere o que a tela consegue conferir; o servidor recusa o resto com `goal_invalid`.
 * `unchangedDue`: na edição, um prazo que não mudou não é recusado, mesmo já tendo passado.
 */
export function validate(f: Form, today: string, opts: { editing?: boolean; unchangedDue?: boolean } = {}): FieldErrors {
  const e: FieldErrors = {}
  const title = f.title.trim()
  if (title.length < 1 || title.length > 120) e.title = 'title'
  if (f.notes.length > 1000) e.notes = 'notes'
  if (f.dueOn && f.dueOn < today && !opts.unchangedDue) e.dueOn = 'dueOnPast'

  if (f.kind === 'habit' || f.kind === 'shared') {
    if (parseTarget(f.target, f.period) === null) e.target = f.period === 'week' ? 'targetWeek' : 'targetMonth'
  }
  if (f.kind === 'financial') {
    const cents = parseMoney(f.amount)
    if (cents === null || cents <= 0 || cents > MAX_CENTS) e.amount = 'amount'
    if (!opts.editing && f.source === 'budget_savings' && !f.categoryId) e.category = 'category'
  }
  if (f.kind === 'project' && !opts.editing && titles(f).some((s) => s.length > 200)) e.steps = 'stepTitle'
  return e
}

/** Corpo do `POST /v1/goals`. Chame só depois de `validate` sem erros. */
export function buildInput(f: Form): GoalInput {
  if (!f.kind || !f.scope) throw new Error('kind e scope são obrigatórios')
  const body: GoalInput = { kind: f.kind, scope: f.scope, title: f.title.trim() }
  if (f.scope === 'personal') body.visibility = f.visibility
  if (f.notes.trim()) body.notes = f.notes.trim()
  if (f.dueOn) body.due_on = f.dueOn

  if (f.kind === 'habit') {
    body.habit = {
      target_count: Number(f.target), period: f.period, weekdays: daysToMask(f.days),
      remind_enabled: f.remindOn, ...(f.remindOn ? { remind_min: timeToMin(f.remindTime) } : {}),
    }
  }
  if (f.kind === 'shared') body.shared = { target_count: Number(f.target), period: f.period, requires_both: f.requiresBoth }
  if (f.kind === 'financial') {
    const budget = f.source === 'budget_savings'
    body.financial = {
      target_cents: parseMoney(f.amount) ?? 0, source: f.source, ...(budget ? { category_id: f.categoryId } : {}),
    }
  }
  if (f.kind === 'project') {
    const task = f.scope === 'household' && f.stepTasks
    body.steps = titles(f).map((title) => (task ? { title, task: true } : { title }))
  }
  return body
}

export function formFromGoal(g: Goal): Form {
  return {
    ...emptyForm,
    kind: g.kind,
    scope: g.scope,
    visibility: g.visibility === 'view' ? 'view' : 'private',
    title: g.title,
    notes: g.notes ?? '',
    dueOn: g.due_on ?? '',
    target: String(g.habit?.target_count ?? g.shared?.target_count ?? emptyForm.target),
    period: g.habit?.period ?? g.shared?.period ?? 'week',
    days: maskToDays(g.habit?.weekdays ?? 0),
    remindOn: g.habit?.remind_enabled ?? false,
    remindTime: g.habit?.remind_enabled ? minToTime(g.habit.remind_min) : emptyForm.remindTime,
    requiresBoth: g.shared?.requires_both ?? false,
    amount: g.financial ? centsToInput(g.financial.target_cents) : '',
    source: g.financial?.source ?? 'contributions',
    categoryId: g.financial?.category_id ?? '',
  }
}

/** Corpo do `PUT /v1/goals/{id}`: `kind`, `period`, `source` e `category_id` não mudam. */
export function buildUpdate(g: Goal, f: Form): GoalUpdate {
  const body: GoalUpdate = { version: g.version, title: f.title.trim() }
  if (f.notes.trim()) body.notes = f.notes.trim()
  if (f.dueOn) body.due_on = f.dueOn
  if (g.scope === 'personal') body.visibility = f.visibility

  if (g.kind === 'habit') {
    body.habit = {
      target_count: Number(f.target), weekdays: daysToMask(f.days), remind_enabled: f.remindOn,
      ...(f.remindOn ? { remind_min: timeToMin(f.remindTime) } : {}),
    }
  }
  if (g.kind === 'shared') body.shared = { target_count: Number(f.target), requires_both: f.requiresBoth }
  if (g.kind === 'financial') body.financial = { target_cents: parseMoney(f.amount) ?? 0 }
  return body
}
