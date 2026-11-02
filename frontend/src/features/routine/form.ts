import type { Block, BlockInput, BlockVisibility, Category } from './api'
import { parseClock, weekdayOf } from './dates'

/** O que a pessoa preenche. Nunca pede a regra de repetição escrita: só opções simples. */
export type Draft = {
  title: string
  category: Category
  location: string
  visibility: BlockVisibility
  recurrence: 'once' | 'weekly'
  weekdays: number[]
  /** 1 = toda semana; 2 = semana sim, semana não */
  intervalWeeks: number
  startDate: string
  /** vazio = sem fim */
  endDate: string
  start: string
  end: string
}

export type Problem = 'title' | 'weekdays' | 'time' | 'dates'

/** Um bloco novo: o padrão da visibilidade é o mais restrito. */
export function emptyDraft(today: string): Draft {
  return {
    title: '',
    category: 'work',
    location: '',
    visibility: 'private',
    recurrence: 'weekly',
    weekdays: [weekdayOf(today)],
    intervalWeeks: 1,
    startDate: today,
    endDate: '',
    start: '09:00',
    end: '10:00',
  }
}

export function draftFromBlock(b: Block): Draft {
  return {
    title: b.title,
    category: b.category,
    location: b.location,
    visibility: b.visibility,
    recurrence: b.recurrence,
    weekdays: b.weekdays ?? [],
    intervalWeeks: b.interval_weeks || 1,
    startDate: b.start_date,
    endDate: b.end_date ?? '',
    start: b.start,
    end: b.end,
  }
}

/** O que impede de salvar. O backend valida de novo, mas isso evita a ida e volta. */
export function problems(d: Draft): Problem[] {
  const out: Problem[] = []
  if (!d.title.trim()) out.push('title')
  if (d.recurrence === 'weekly' && d.weekdays.length === 0) out.push('weekdays')
  // o fim deve ser depois do início e não cruza a meia-noite ("24:00" é permitido)
  if (!d.start || !d.end || parseClock(d.end) <= parseClock(d.start)) out.push('time')
  if (!d.startDate || (d.endDate && d.endDate < d.startDate)) out.push('dates')
  return out
}

export function toBody(d: Draft): BlockInput {
  const weekly = d.recurrence === 'weekly'
  return {
    title: d.title.trim(),
    category: d.category,
    location: d.location.trim(),
    visibility: d.visibility,
    recurrence: d.recurrence,
    weekdays: weekly ? [...d.weekdays].sort((a, b) => a - b) : [],
    interval_weeks: weekly ? d.intervalWeeks : 1,
    start_date: d.startDate,
    end_date: weekly ? d.endDate : '',
    start: d.start,
    end: d.end,
  }
}

/** Liga ou desliga um dia da semana, mantendo a lista sem repetir. */
export function toggleWeekday(days: number[], day: number): number[] {
  return days.includes(day) ? days.filter((d) => d !== day) : [...days, day]
}
