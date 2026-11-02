import { describe, expect, it } from 'vitest'
import { budgetTone, compare } from './summary'

describe('orçamento', () => {
  it.each([
    [0, 'neutral'],
    [79, 'neutral'],
    [80, 'attention'],
    [99, 'attention'],
    [100, 'alert'],
    [250, 'alert'],
  ])('%i%% é %s', (percent, tone) => {
    expect(budgetTone(percent)).toBe(tone)
  })
})

describe('comparação com o mês anterior', () => {
  it('em centavos e em %, sem ponto flutuante', () => {
    expect(compare(120000, 100000)).toEqual({ kind: 'more', diff: 20000, percent: 20 })
    expect(compare(50000, 100000)).toEqual({ kind: 'less', diff: 50000, percent: 50 })
    expect(compare(100, 100)).toEqual({ kind: 'same' })
  })

  it('sem mês anterior não há porcentagem', () => {
    expect(compare(5000, 0)).toEqual({ kind: 'more', diff: 5000, percent: null })
  })
})
