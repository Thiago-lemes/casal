import { describe, expect, it } from 'vitest'
import type { Block } from './api'
import { draftFromBlock, emptyDraft, problems, toBody, toggleWeekday } from './form'

const today = '2026-11-04' // quarta (3)

describe('emptyDraft', () => {
  it('começa privado, semanal e no dia da semana de hoje', () => {
    const d = emptyDraft(today)
    expect(d).toMatchObject({ visibility: 'private', recurrence: 'weekly', weekdays: [3], intervalWeeks: 1, startDate: today })
  })
})

describe('problems', () => {
  const ok = { ...emptyDraft(today), title: 'Trabalho' }

  it('um bloco completo não tem problemas', () => {
    expect(problems(ok)).toEqual([])
  })

  it('título vazio ou só espaços', () => {
    expect(problems({ ...ok, title: '   ' })).toEqual(['title'])
  })

  it('semanal sem nenhum dia marcado', () => {
    expect(problems({ ...ok, weekdays: [] })).toEqual(['weekdays'])
    expect(problems({ ...ok, recurrence: 'once', weekdays: [] })).toEqual([]) // uma vez não precisa de dia da semana
  })

  it('o fim precisa ser depois do início, e 24:00 vale', () => {
    expect(problems({ ...ok, start: '10:00', end: '10:00' })).toEqual(['time'])
    expect(problems({ ...ok, start: '10:00', end: '09:00' })).toEqual(['time'])
    expect(problems({ ...ok, start: '22:00', end: '24:00' })).toEqual([])
  })

  it('a data final não pode ser antes da inicial', () => {
    expect(problems({ ...ok, endDate: '2026-11-01' })).toEqual(['dates'])
    expect(problems({ ...ok, endDate: '2026-12-01' })).toEqual([])
  })
})

describe('toBody', () => {
  it('semanal: ordena os dias e apara os textos', () => {
    const body = toBody({ ...emptyDraft(today), title: ' Academia ', location: ' Smart ', weekdays: [5, 1, 3], intervalWeeks: 2, endDate: '2027-01-01' })
    expect(body).toMatchObject({
      title: 'Academia',
      location: 'Smart',
      recurrence: 'weekly',
      weekdays: [1, 3, 5],
      interval_weeks: 2,
      end_date: '2027-01-01',
    })
  })

  it('uma vez: sem dias, sem intervalo e sem data final', () => {
    const body = toBody({ ...emptyDraft(today), title: 'Dentista', recurrence: 'once', weekdays: [3], intervalWeeks: 3, endDate: '2027-01-01' })
    expect(body).toMatchObject({ weekdays: [], interval_weeks: 1, end_date: '' })
  })
})

describe('draftFromBlock', () => {
  it('lê um bloco com campos nulos do backend', () => {
    const block: Block = {
      id: 'b',
      owner_id: 'me',
      owner_name: 'Ana',
      title: 'Estudo',
      category: 'study',
      location: '',
      visibility: 'busy',
      busy: false,
      recurrence: 'once',
      weekdays: null,
      interval_weeks: 0,
      start_date: '2026-11-04',
      end_date: null,
      start: '19:00',
      end: '21:00',
      version: 2,
    }
    expect(draftFromBlock(block)).toMatchObject({ weekdays: [], intervalWeeks: 1, endDate: '', visibility: 'busy' })
  })
})

describe('toggleWeekday', () => {
  it('liga e desliga sem repetir', () => {
    expect(toggleWeekday([1], 3)).toEqual([1, 3])
    expect(toggleWeekday([1, 3], 3)).toEqual([1])
  })
})
