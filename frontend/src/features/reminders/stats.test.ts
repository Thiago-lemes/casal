import { describe, expect, it } from 'vitest'
import type { Reminder } from './api'
import { busyDays, dayKey, dayStats, weekSeries } from './stats'

const now = new Date(2026, 10, 1, 15, 0)
const at = (d: number, h: number) => new Date(2026, 10, d, h).toISOString()

const r = (over: Partial<Reminder>): Reminder => ({
  id: Math.random().toString(),
  created_by: 'a',
  created_by_name: 'Ana',
  recipient_id: 'a',
  body: 'x',
  remind_at: at(1, 18),
  status: 'scheduled',
  ...over,
})

describe('dayStats', () => {
  it('sem nada para hoje, o progresso é null (não 0%)', () => {
    expect(dayStats([], now).progress).toBeNull()
    expect(dayStats([r({ remind_at: at(5, 9) })], now).progress).toBeNull()
  })

  it('conta pendentes, atrasados e baixas de hoje', () => {
    const all = [
      r({ remind_at: at(1, 10) }), // atrasado
      r({ remind_at: at(1, 20) }), // hoje, ainda não venceu
      r({ remind_at: at(4, 9) }), // outro dia: pendente, mas não é de hoje
      r({ status: 'resolved', resolved_at: at(1, 9) }),
      r({ status: 'resolved', resolved_at: at(1, 12) }),
      r({ status: 'resolved', resolved_at: at(0, 12) }), // outubro
    ]
    expect(dayStats(all, now)).toEqual({ pending: 3, overdue: 1, doneToday: 2, dueToday: 2, progress: 0.5 })
  })
})

describe('weekSeries', () => {
  it('devolve 7 dias terminando hoje, com as baixas de cada um', () => {
    const w = weekSeries(
      [r({ status: 'resolved', resolved_at: at(1, 9) }), r({ status: 'resolved', resolved_at: at(0, 9) })],
      now,
    )
    expect(w).toHaveLength(7)
    expect(w[6]).toMatchObject({ today: true, done: 1 })
    expect(w[0].key).toBe(dayKey(new Date(2026, 9, 26)))
    expect(w[5].done).toBe(1) // 31 de outubro
  })
})

describe('busyDays', () => {
  it('marca só os dias com pendência', () => {
    const set = busyDays([r({ remind_at: at(4, 9) }), r({ status: 'resolved', remind_at: at(6, 9) })])
    expect([...set]).toEqual(['2026-11-04'])
  })
})
