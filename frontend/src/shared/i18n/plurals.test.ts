import { describe, expect, it } from 'vitest'
import i18n from './index'

const t = i18n.getFixedT('pt-BR', 'finance')

describe('plural das parcelas', () => {
  it('concorda no singular e no plural', () => {
    expect(t('group.confirmDelete', { count: 1 })).toBe('Apagar a parcela aberta?')
    expect(t('group.confirmDelete', { count: 3 })).toBe('Apagar as 3 parcelas abertas?')
    expect(t('group.confirmKept', { count: 1 })).toBe('A parcela já acertada fica.')
    expect(t('group.confirmKept', { count: 2 })).toBe('As 2 já acertadas ficam.')
    expect(t('group.deleted', { count: 1 })).toBe('Parcela apagada.')
    expect(t('group.deleted', { count: 4 })).toBe('4 parcelas apagadas.')
    expect(t('group.remaining', { count: 1 })).toBe('1 parcela já acertada ficou.')
    expect(t('group.remaining', { count: 2 })).toBe('2 parcelas já acertadas ficaram.')
  })
})

describe('vocabulário', () => {
  it('a outra pessoa não é chamada de parceiro nem de par nos textos pt-BR', async () => {
    const files = import.meta.glob('./locales/pt-BR/*.json', { eager: true, import: 'default' })
    // só os textos (valores), nunca as chaves;  evita pegar "seu parcelamento"
    const texts: string[] = []
    const walk = (v: unknown): void => {
      if (typeof v === 'string') texts.push(v)
      else if (v && typeof v === 'object') Object.values(v).forEach(walk)
    }
    walk(files)
    expect(texts.length).toBeGreaterThan(100)
    expect(texts.filter((s) => /(parceir[oa]s?|seu par|meu par|a(o|s)? par)/i.test(s))).toEqual([])
  })
})
