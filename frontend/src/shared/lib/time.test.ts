import { describe, expect, it } from 'vitest'
import { minutesUntilTomorrowMorning, quickTimes } from './time'

describe('quickTimes', () => {
  it('"hoje à noite" vira amanhã se já passou das 20h', () => {
    const now = new Date(2026, 10, 1, 21, 0)
    expect(quickTimes(now).tonight.getDate()).toBe(2)
  })

  it('"hoje à noite" é hoje às 20h antes disso', () => {
    const now = new Date(2026, 10, 1, 14, 0)
    const t = quickTimes(now).tonight
    expect([t.getDate(), t.getHours()]).toEqual([1, 20])
  })

  it('amanhã de manhã é 8h do dia seguinte', () => {
    const t = quickTimes(new Date(2026, 10, 1, 23, 30)).tomorrowMorning
    expect([t.getDate(), t.getHours()]).toEqual([2, 8])
  })

  it('o adiamento até amanhã respeita o limite de 1440 minutos do servidor', () => {
    expect(minutesUntilTomorrowMorning(new Date(2026, 10, 1, 8, 1))).toBeLessThanOrEqual(1440)
  })
})
