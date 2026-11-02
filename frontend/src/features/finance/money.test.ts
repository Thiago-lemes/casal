import { describe, expect, it } from 'vitest'
import { centsToInput, formatBRL, installmentPreview, parseMoney, splitLabel } from './money'

describe('parseMoney', () => {
  it.each([
    ['1.234,56', 123456],
    ['1234,56', 123456],
    ['1234.56', 123456],
    ['12', 1200],
    ['0,5', 50],
    ['0,05', 5],
    ['1.234', 123400],
    ['1.234.567,89', 123456789],
    ['R$ 10,00', 1000],
    ['  7,5 ', 750],
    ['0,1', 10],
    ['19,99', 1999],
    ['1.1', 110],
  ])('%s → %i centavos', (text, cents) => {
    expect(parseMoney(text)).toBe(cents)
  })

  it('0,1 + 0,2 dá exatamente 30 centavos, sem erro de ponto flutuante', () => {
    expect(parseMoney('0,1')! + parseMoney('0,2')!).toBe(30)
    expect(parseMoney('4,35')).toBe(435) // parseFloat('4.35') * 100 = 434.99999...
    expect(parseMoney('1,15')).toBe(115)
  })

  it.each(['', '  ', 'abc', '-5', '1,2,3', '12,345', '1.23.4', '0.123', '1..2', ',5', '.', '12a', '1.2345'])(
    'recusa %j',
    (text) => {
      expect(parseMoney(text)).toBeNull()
    },
  )
})

describe('formatBRL / splitLabel / afins', () => {
  it('formata em reais', () => {
    expect(formatBRL(123456).replace(/\s/g, ' ')).toBe('R$ 1.234,56')
    expect(formatBRL(0).replace(/\s/g, ' ')).toBe('R$ 0,00')
  })

  it('pontos-base viram porcentagem', () => {
    expect(splitLabel(6250)).toBe('62,5%')
    expect(splitLabel(5000)).toBe('50%')
    expect(splitLabel(10000)).toBe('100%')
  })

  it('pré-visualização de parcelas', () => {
    expect(installmentPreview(30000, 3)).toEqual({ each: 10000, exact: true })
    expect(installmentPreview(10000, 3)).toEqual({ each: 3333, exact: false })
  })

  it('centavos voltam como texto editável', () => {
    expect(centsToInput(123456)).toBe('1234,56')
    expect(centsToInput(1200)).toBe('12')
    expect(centsToInput(5)).toBe('0,05')
    expect(parseMoney(centsToInput(123456))).toBe(123456)
  })
})
