import type { Occurrence } from './api'
import { dayKeyIn, minutesOfDay } from './dates'

export type Placed = {
  occ: Occurrence
  /** minutos desde a meia-noite do dia visto (0 a 1440), já recortados ao dia */
  from: number
  to: number
  /** faixa dentro de um grupo que se sobrepõe, e quantas faixas o grupo tem */
  lane: number
  lanes: number
}

/** Os minutos de início e fim de uma ocorrência dentro de um dia (recorta o que atravessa a meia-noite). */
export function minutesInDay(occ: Occurrence, day: string, timeZone: string): { from: number; to: number } | null {
  const startDay = dayKeyIn(occ.start_at, timeZone)
  const endDay = dayKeyIn(new Date(new Date(occ.end_at).getTime() - 1), timeZone)
  if (day < startDay || day > endDay) return null
  const from = day === startDay ? minutesOfDay(occ.start_at, timeZone) : 0
  const to = day === endDay ? minutesOfDay(occ.end_at, timeZone) || 1440 : 1440
  return to > from ? { from, to } : null
}

/**
 * Posiciona as ocorrências de um dia numa linha do tempo. As que se sobrepõem dividem a largura
 * em faixas (lanes), para nenhuma esconder a outra.
 */
export function placeDay(occs: Occurrence[], day: string, timeZone: string): Placed[] {
  const items = occs
    .map((occ) => ({ occ, span: minutesInDay(occ, day, timeZone) }))
    .filter((x): x is { occ: Occurrence; span: { from: number; to: number } } => x.span !== null)
    .sort((a, b) => a.span.from - b.span.from || a.span.to - b.span.to)

  const placed: Placed[] = []
  let group: Placed[] = []
  let groupEnd = -1
  let laneEnds: number[] = []

  const flush = () => {
    for (const p of group) p.lanes = laneEnds.length
    placed.push(...group)
    group = []
    laneEnds = []
    groupEnd = -1
  }

  for (const { occ, span } of items) {
    if (span.from >= groupEnd) flush()
    let lane = laneEnds.findIndex((end) => end <= span.from)
    if (lane === -1) {
      lane = laneEnds.length
      laneEnds.push(span.to)
    } else {
      laneEnds[lane] = span.to
    }
    group.push({ occ, from: span.from, to: span.to, lane, lanes: 1 })
    groupEnd = Math.max(groupEnd, span.to)
  }
  flush()
  return placed
}

/** O trecho de horas que a linha do tempo mostra: o do dia útil, ampliado se houver algo fora dele. */
export function visibleRange(placed: Placed[], defaultFrom = 7 * 60, defaultTo = 22 * 60): { from: number; to: number } {
  let from = defaultFrom
  let to = defaultTo
  for (const p of placed) {
    from = Math.min(from, Math.floor(p.from / 60) * 60)
    to = Math.max(to, Math.ceil(p.to / 60) * 60)
  }
  return { from, to }
}
