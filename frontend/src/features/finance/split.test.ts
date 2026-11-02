import { describe, expect, it } from 'vitest'
import { allOn, equalSplit, setShare, splitTotal, WHOLE } from './split'

const people = [
  { id: 'a', name: 'Ana' },
  { id: 'b', name: 'Bia' },
]

describe('divisão', () => {
  it('mudar a parte de um completa a do outro: a soma é sempre 10000', () => {
    for (const p of [0, 1, 3333, 5000, 6250, 9999, 10000, -50, 12000]) {
      expect(splitTotal(setShare(people, 'a', p))).toBe(WHOLE)
    }
    expect(setShare(people, 'a', 6250)).toEqual({ a: 6250, b: 3750 })
  })

  it('igual soma 10000 também com número ímpar de pessoas', () => {
    expect(equalSplit(people)).toEqual({ a: 5000, b: 5000 })
    const three = [...people, { id: 'c', name: 'Cris' }]
    expect(splitTotal(equalSplit(three))).toBe(WHOLE)
  })

  it('casal de uma pessoa só ou vazio nunca gera NaN nem soma torta', () => {
    expect(setShare([people[0]], 'a', 3000)).toEqual({ a: 10000 })
    expect(equalSplit([])).toEqual({})
    expect(equalSplit([people[0]])).toEqual({ a: 10000 })
  })

  it('"eu pago tudo" põe 100% em quem paga e 0 no outro', () => {
    expect(allOn(people, 'a')).toEqual({ a: 10000, b: 0 })
  })
})
