import { describe, expect, it } from 'vitest'
import { addDays, clock, dayKeyIn, duration, minutesOfDay, parseClock, weekDays, weekdayOf, weekStart } from './dates'

describe('dias', () => {
  it('o dia de um instante depende do fuso, não do aparelho', () => {
    const t = '2026-11-02T01:30:00Z' // ainda é domingo em São Paulo (UTC-3)
    expect(dayKeyIn(t, 'America/Sao_Paulo')).toBe('2026-11-01')
    expect(dayKeyIn(t, 'Europe/Lisbon')).toBe('2026-11-02')
    expect(dayKeyIn(t, 'Asia/Tokyo')).toBe('2026-11-02')
  })

  it('soma dias atravessando mês e ano', () => {
    expect(addDays('2026-11-30', 1)).toBe('2026-12-01')
    expect(addDays('2026-12-31', 1)).toBe('2027-01-01')
    expect(addDays('2026-03-01', -1)).toBe('2026-02-28')
  })

  it('segunda é 1 e domingo é 7, como no backend', () => {
    expect(weekdayOf('2026-11-02')).toBe(1)
    expect(weekdayOf('2026-11-08')).toBe(7)
    expect(weekdayOf('2026-11-01')).toBe(7)
  })

  it('a semana vai de segunda a domingo', () => {
    expect(weekStart('2026-11-01')).toBe('2026-10-26') // domingo pertence à semana que começou na segunda anterior
    expect(weekDays('2026-11-04')).toEqual(['2026-11-02', '2026-11-03', '2026-11-04', '2026-11-05', '2026-11-06', '2026-11-07', '2026-11-08'])
  })
})

describe('horas', () => {
  it('minutos desde a meia-noite, no fuso', () => {
    expect(minutesOfDay('2026-11-02T12:30:00Z', 'America/Sao_Paulo')).toBe(9 * 60 + 30)
    expect(minutesOfDay('2026-11-02T03:00:00Z', 'America/Sao_Paulo')).toBe(0) // meia-noite local não vira 24
  })

  it('converte "HH:MM" e minutos, inclusive "24:00"', () => {
    expect(parseClock('07:05')).toBe(425)
    expect(parseClock('24:00')).toBe(1440)
    expect(clock(425)).toBe('07:05')
    expect(clock(1440)).toBe('24:00')
  })

  it('duração legível', () => {
    expect(duration(90)).toBe('1 h 30 min')
    expect(duration(120)).toBe('2 h')
    expect(duration(45)).toBe('45 min')
  })
})
