import { describe, expect, it } from 'vitest'
import { resources } from './resources'

type Tree = { [k: string]: string | Tree }

function keys(tree: Tree, prefix = ''): string[] {
  return Object.entries(tree).flatMap(([k, v]) =>
    typeof v === 'string' ? [prefix + k] : keys(v, `${prefix}${k}.`),
  )
}

// plurais mudam o sufixo entre idiomas (_one/_other); comparamos pela raiz
const root = (k: string) => k.replace(/_(one|other|few|many|zero|two)$/, '')

describe('traduções', () => {
  for (const ns of Object.keys(resources['pt-BR']) as (keyof (typeof resources)['pt-BR'])[]) {
    it(`pt-BR e en têm as mesmas chaves em "${ns}"`, () => {
      const pt = new Set(keys(resources['pt-BR'][ns] as Tree).map(root))
      const en = new Set(keys(resources.en[ns] as Tree).map(root))
      expect([...pt].filter((k) => !en.has(k))).toEqual([])
      expect([...en].filter((k) => !pt.has(k))).toEqual([])
    })
  }
})
