import type { Split } from './api'
import type { Person } from './types'

/** Pontos-base: 1% = 100, o todo é 10000. */
export const WHOLE = 10000

/** Muda a parte de uma pessoa e devolve a da outra, para a soma ficar sempre em 10000. */
export function setShare(people: Person[], id: string, points: number): Split {
  const p = Math.max(0, Math.min(WHOLE, Math.round(points)))
  const other = people.find((x) => x.id !== id)
  // casal de uma pessoa só: ela leva o todo, para a soma fechar em 10000
  if (!other) return { [id]: WHOLE }
  return { [id]: p, [other.id]: WHOLE - p }
}

/** Divisão igual entre as pessoas; a sobra de pontos fica com a primeira. */
export function equalSplit(people: Person[]): Split {
  if (people.length === 0) return {}
  const base = Math.floor(WHOLE / people.length)
  return Object.fromEntries(people.map((p, i) => [p.id, i === 0 ? WHOLE - base * (people.length - 1) : base]))
}

export const splitTotal = (s: Split) => Object.values(s).reduce((a, b) => a + b, 0)

/** "Eu pago tudo": o outro não deve nada (0/100 para o parceiro). */
export function allOn(people: Person[], payerId: string): Split {
  return Object.fromEntries(people.map((p) => [p.id, p.id === payerId ? WHOLE : 0]))
}

export type SplitMode = 'default' | 'equal' | 'mine' | 'theirs' | 'custom'

/** A divisão a enviar para cada escolha; `undefined` deixa o servidor aplicar a divisão padrão do casal. */
export function splitFor(mode: SplitMode, people: Person[], meId: string, custom: Split): Split | undefined {
  const partner = people.find((p) => p.id !== meId)
  switch (mode) {
    case 'default':
      return undefined
    case 'equal':
      return equalSplit(people)
    case 'mine':
      return allOn(people, meId)
    case 'theirs':
      return partner ? allOn(people, partner.id) : undefined
    case 'custom':
      return custom
  }
}
