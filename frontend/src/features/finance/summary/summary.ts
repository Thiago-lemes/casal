export type Tone = 'neutral' | 'attention' | 'alert'

/** Neutra até 79%, atenção de 80% a 99%, alerta a partir de 100%. */
export function budgetTone(usedPercent: number): Tone {
  if (usedPercent >= 100) return 'alert'
  if (usedPercent >= 80) return 'attention'
  return 'neutral'
}

export type Comparison = { kind: 'same' } | { kind: 'more' | 'less'; diff: number; percent: number | null }

/** Total contra o mês anterior, em inteiros: a diferença em centavos e em % (sem % se o mês anterior foi zero). */
export function compare(total: number, previous: number): Comparison {
  if (total === previous) return { kind: 'same' }
  const diff = Math.abs(total - previous)
  return {
    kind: total > previous ? 'more' : 'less',
    diff,
    percent: previous > 0 ? Math.round((diff * 100) / previous) : null,
  }
}
