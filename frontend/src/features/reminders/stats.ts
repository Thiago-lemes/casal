import { isPending } from './api'
import type { Reminder } from './api'

/** Chave AAAA-MM-DD no fuso do aparelho. */
export const dayKey = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`

const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate())

export type DayStats = {
  pending: number
  overdue: number
  doneToday: number
  /** pendentes com horário até o fim de hoje */
  dueToday: number
  /** 0 a 1; null quando não há nada para hoje (evita "0%" enganoso) */
  progress: number | null
}

/** Números do dia. "Pendente de hoje" = pendente com horário até o fim de hoje. */
export function dayStats(all: Reminder[], now = new Date()): DayStats {
  const endOfToday = new Date(startOfDay(now).getTime() + 24 * 3600 * 1000)
  const pending = all.filter(isPending)
  const dueToday = pending.filter((r) => new Date(r.remind_at) < endOfToday)
  const doneToday = all.filter(
    (r) => r.status === 'resolved' && r.resolved_at && dayKey(new Date(r.resolved_at)) === dayKey(now),
  ).length
  const total = doneToday + dueToday.length

  return {
    pending: pending.length,
    overdue: pending.filter((r) => new Date(r.remind_at) < now).length,
    doneToday,
    dueToday: dueToday.length,
    progress: total === 0 ? null : doneToday / total,
  }
}

export type WeekDay = { key: string; date: Date; done: number; today: boolean }

/** Baixas por dia nos últimos 7 dias, do mais antigo ao de hoje. */
export function weekSeries(all: Reminder[], now = new Date()): WeekDay[] {
  const counts = new Map<string, number>()
  for (const r of all) {
    if (r.status === 'resolved' && r.resolved_at) {
      const k = dayKey(new Date(r.resolved_at))
      counts.set(k, (counts.get(k) ?? 0) + 1)
    }
  }
  return Array.from({ length: 7 }, (_, i) => {
    // reconstrói pelo calendário para não derivar com horário de verão
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() - (6 - i))
    return { key: dayKey(d), date: d, done: counts.get(dayKey(d)) ?? 0, today: i === 6 }
  })
}

/** Dias (AAAA-MM-DD) que têm lembrete pendente. */
export function busyDays(all: Reminder[]): Set<string> {
  return new Set(all.filter(isPending).map((r) => dayKey(new Date(r.remind_at))))
}
