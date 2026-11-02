import { describe, expect, it } from 'vitest'
import { resources } from '@/shared/i18n/resources'

type Tree = { [k: string]: string | Tree }

function texts(tree: Tree): string[] {
  return Object.values(tree).flatMap((v) => (typeof v === 'string' ? [v] : texts(v)))
}

// Metas é sem ranking e sem cobrança: nenhum texto pode acusar atraso, falha, perda, sequência ou comparação.
const BANNED: Record<'pt-BR' | 'en', RegExp> = {
  'pt-BR': /atrasad|falhou|falhas?\b|perdeu|sequência|ranking|vermelh|cobran/i,
  en: /\b(late|overdue|failed|failure|lost|streak|ranking|behind|missed|red)\b/i,
}

describe('neutralidade dos textos de Metas', () => {
  for (const lang of ['pt-BR', 'en'] as const) {
    it(`${lang}: nenhuma palavra de cobrança`, () => {
      const found = texts(resources[lang].goals as Tree).filter((s) => BANNED[lang].test(s))
      expect(found).toEqual([])
    })
  }

  it('nenhum texto compara as duas pessoas', () => {
    const all = [...texts(resources['pt-BR'].goals as Tree), ...texts(resources.en.goals as Tree)]
    expect(all.filter((s) => /mais que (o|a) (outr|par)|more than (your )?partner|quem fez mais|who did more/i.test(s))).toEqual([])
  })
})
