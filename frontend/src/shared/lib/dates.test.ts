import { describe, expect, it } from 'vitest'
import { daysBetween, formatDay, monthOf, parseDay, shiftMonth, toDay, todayIn } from './dates'

describe('datas sem fuso', () => {
  it('"2026-11-15" é dia 15 no calendário local, nunca o dia anterior', () => {
    const d = parseDay('2026-11-15')
    expect([d.getFullYear(), d.getMonth(), d.getDate()]).toEqual([2026, 10, 15])
    expect(toDay(d)).toBe('2026-11-15')
    expect(formatDay('2026-11-15', 'pt-BR', { day: 'numeric' })).toBe('15')
  })

  it('"hoje" vem do fuso da pessoa, não do aparelho', () => {
    const instant = new Date('2026-11-02T01:30:00Z')
    expect(todayIn('America/Sao_Paulo', instant)).toBe('2026-11-01')
    expect(todayIn('Asia/Tokyo', instant)).toBe('2026-11-02')
  })

  it('fuso vazio ou inválido não quebra: cai no do aparelho', () => {
    const instant = new Date('2026-11-02T12:00:00Z')
    expect(todayIn('', instant)).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(todayIn('Nao/Existe', instant)).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })

  it('meses andam por cima da virada de ano', () => {
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
    expect(monthOf('2026-11-15')).toBe('2026-11')
  })

  it('conta dias entre datas', () => {
    expect(daysBetween('2026-11-01', '2026-11-04')).toBe(3)
    expect(daysBetween('2026-11-04', '2026-11-01')).toBe(-3)
  })
})
