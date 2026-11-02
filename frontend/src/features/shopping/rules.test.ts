import { describe, expect, it } from 'vitest'
import type { Item } from './api'
import { boughtRecently, formatQuantity, groupOpen, parseQuantity } from './rules'

const item = (name: string, over: Partial<Item> = {}): Item => ({
  id: name,
  name,
  quantity: null,
  unit: '',
  category: 'other',
  recurring: false,
  source: 'manual',
  added_by: 'me',
  created_at: '2026-11-01T10:00:00Z',
  ...over,
})

describe('groupOpen', () => {
  it('agrupa na ordem do mercado, ordena por nome e esconde categoria vazia e comprados', () => {
    const groups = groupOpen([
      item('Sabão', { category: 'cleaning' }),
      item('Maçã', { category: 'produce' }),
      item('Alface', { category: 'produce' }),
      item('Arroz', { category: 'grocery' }),
      item('Leite', { category: 'grocery', bought_at: '2026-11-01T12:00:00Z' }),
    ])
    expect(groups.map((g) => g.category)).toEqual(['produce', 'grocery', 'cleaning'])
    expect(groups[0].items.map((i) => i.name)).toEqual(['Alface', 'Maçã'])
    expect(groups[1].items.map((i) => i.name)).toEqual(['Arroz'])
  })

  it('lista vazia não tem grupos', () => {
    expect(groupOpen([])).toEqual([])
  })
})

describe('boughtRecently', () => {
  it('só os comprados, o mais novo primeiro', () => {
    const out = boughtRecently([
      item('a', { bought_at: '2026-11-01T10:00:00Z' }),
      item('b'),
      item('c', { bought_at: '2026-11-01T14:00:00Z' }),
    ])
    expect(out.map((i) => i.name)).toEqual(['c', 'a'])
  })
})

describe('parseQuantity', () => {
  it('vazio é "sem quantidade"; aceita vírgula e ponto', () => {
    expect(parseQuantity('')).toBeNull()
    expect(parseQuantity('  ')).toBeNull()
    expect(parseQuantity('2')).toBe(2)
    expect(parseQuantity('1,5')).toBe(1.5)
    expect(parseQuantity('0.25')).toBe(0.25)
  })

  it('texto, zero e negativo são inválidos', () => {
    expect(parseQuantity('abc')).toBeUndefined()
    expect(parseQuantity('0')).toBeUndefined()
    expect(parseQuantity('-1')).toBeUndefined()
    expect(parseQuantity('1,2,3')).toBeUndefined()
  })
})

describe('formatQuantity', () => {
  it('no idioma da interface, com a unidade quando houver', () => {
    expect(formatQuantity(1.5, 'kg', 'pt-BR')).toBe('1,5 kg')
    expect(formatQuantity(1.5, 'kg', 'en')).toBe('1.5 kg')
    expect(formatQuantity(3, '', 'pt-BR')).toBe('3')
  })

  it('sem quantidade não mostra nada, nem a unidade', () => {
    expect(formatQuantity(null, 'kg', 'pt-BR')).toBe('')
    expect(formatQuantity(undefined, '', 'pt-BR')).toBe('')
  })
})
