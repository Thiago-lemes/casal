import { describe, expect, it } from 'vitest'
import type { Occurrence } from './api'
import { minutesInDay, placeDay, visibleRange } from './layout'

const TZ = 'America/Sao_Paulo' // UTC-3, sem horário de verão em 2026
// 09:00 locais = 12:00Z
const at = (day: number, hh: number, mm = 0) => new Date(Date.UTC(2026, 10, day, hh + 3, mm)).toISOString()

const occ = (id: string, start: string, end: string, over: Partial<Occurrence> = {}): Occurrence => ({
  block_id: id,
  owner_id: 'me',
  owner_name: 'Ana',
  title: id,
  category: 'work',
  location: '',
  busy: false,
  start_at: start,
  end_at: end,
  exception: false,
  ...over,
})

describe('minutesInDay', () => {
  it('converte para minutos do dia no fuso', () => {
    expect(minutesInDay(occ('a', at(2, 9), at(2, 10, 30)), '2026-11-02', TZ)).toEqual({ from: 540, to: 630 })
  })

  it('fora do dia, devolve nulo', () => {
    expect(minutesInDay(occ('a', at(2, 9), at(2, 10)), '2026-11-03', TZ)).toBeNull()
  })

  it('termina à meia-noite: o fim é 1440, não 0', () => {
    expect(minutesInDay(occ('a', at(2, 22), at(3, 0)), '2026-11-02', TZ)).toEqual({ from: 1320, to: 1440 })
  })

  it('atravessa a meia-noite: recorta em cada dia', () => {
    const o = occ('a', at(2, 22), at(3, 2))
    expect(minutesInDay(o, '2026-11-02', TZ)).toEqual({ from: 1320, to: 1440 })
    expect(minutesInDay(o, '2026-11-03', TZ)).toEqual({ from: 0, to: 120 })
  })
})

describe('placeDay', () => {
  it('sem sobreposição, tudo na faixa 0', () => {
    const out = placeDay([occ('a', at(2, 9), at(2, 10)), occ('b', at(2, 11), at(2, 12))], '2026-11-02', TZ)
    expect(out.map((p) => [p.lane, p.lanes])).toEqual([[0, 1], [0, 1]])
  })

  it('duas que se sobrepõem dividem em duas faixas', () => {
    const out = placeDay([occ('a', at(2, 9), at(2, 11)), occ('b', at(2, 10), at(2, 12))], '2026-11-02', TZ)
    expect(out.map((p) => [p.occ.block_id, p.lane, p.lanes])).toEqual([['a', 0, 2], ['b', 1, 2]])
  })

  it('uma faixa livre é reaproveitada, e o grupo seguinte volta a ter uma faixa só', () => {
    const out = placeDay(
      [occ('a', at(2, 9), at(2, 10)), occ('b', at(2, 9, 30), at(2, 11)), occ('c', at(2, 10), at(2, 11)), occ('d', at(2, 15), at(2, 16))],
      '2026-11-02',
      TZ,
    )
    const byId = Object.fromEntries(out.map((p) => [p.occ.block_id, [p.lane, p.lanes]]))
    expect(byId).toEqual({ a: [0, 2], b: [1, 2], c: [0, 2], d: [0, 1] })
  })

  it('ignora o que é de outro dia', () => {
    expect(placeDay([occ('a', at(3, 9), at(3, 10))], '2026-11-02', TZ)).toEqual([])
  })
})

describe('visibleRange', () => {
  it('usa 7h às 22h por padrão', () => {
    expect(visibleRange([])).toEqual({ from: 420, to: 1320 })
  })

  it('amplia para incluir o que está fora (madrugada ou noite)', () => {
    const placed = placeDay([occ('a', at(2, 5, 30), at(2, 6)), occ('b', at(2, 22), at(2, 23, 30))], '2026-11-02', TZ)
    expect(visibleRange(placed)).toEqual({ from: 300, to: 1440 })
  })
})
