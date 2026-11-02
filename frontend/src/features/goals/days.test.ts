import { describe, expect, it } from 'vitest'
import { addDays, inMarkWindow, pastDays } from './days'

describe('days', () => {
  it('soma dias atravessando mês e ano, sem fuso', () => {
    expect(addDays('2026-11-01', -1)).toBe('2026-10-31')
    expect(addDays('2026-12-31', 1)).toBe('2027-01-01')
    expect(addDays('2026-03-01', -1)).toBe('2026-02-28')
  })

  it('31 dias para trás, do mais recente ao mais antigo, sem hoje', () => {
    const days = pastDays('2026-11-02')
    expect(days).toHaveLength(31)
    expect(days[0]).toBe('2026-11-01')
    expect(days[30]).toBe('2026-10-02')
  })

  it('a janela de marcação vai de 31 dias atrás até hoje', () => {
    expect(inMarkWindow('2026-11-02', '2026-11-02')).toBe(true)
    expect(inMarkWindow('2026-10-02', '2026-11-02')).toBe(true)
    expect(inMarkWindow('2026-10-01', '2026-11-02')).toBe(false)
    expect(inMarkWindow('2026-11-03', '2026-11-02')).toBe(false)
  })
})
