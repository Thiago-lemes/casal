import { describe, expect, it } from 'vitest'
import { allowsDay, daysToMask, isoWeekday, maskToDays } from './weekdays'

describe('weekdays', () => {
  it('segunda e quarta viram 0b101 e voltam', () => {
    expect(daysToMask([1, 3])).toBe(0b0000101)
    expect(maskToDays(0b0000101)).toEqual([1, 3])
  })

  it('domingo é o bit 6', () => {
    expect(daysToMask([7])).toBe(0b1000000)
    expect(maskToDays(0b1000000)).toEqual([7])
  })

  it('máscara 0 é qualquer dia; lista vazia vira 0', () => {
    expect(daysToMask([])).toBe(0)
    expect(maskToDays(0)).toEqual([])
    expect(allowsDay(0, '2026-11-01')).toBe(true)
  })

  it('ignora dias fora de 1 a 7 e a ordem de entrada', () => {
    expect(daysToMask([9, 0, 5, 1])).toBe(0b0010001)
    expect(maskToDays(daysToMask([6, 2]))).toEqual([2, 6])
  })

  it('o dia da semana vem do calendário, sem fuso', () => {
    expect(isoWeekday('2026-11-01')).toBe(7) // domingo
    expect(isoWeekday('2026-11-02')).toBe(1) // segunda
    expect(allowsDay(daysToMask([1]), '2026-11-02')).toBe(true)
    expect(allowsDay(daysToMask([1]), '2026-11-03')).toBe(false)
  })
})
