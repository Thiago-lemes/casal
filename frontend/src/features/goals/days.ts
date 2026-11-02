import { parseDay, toDay } from '@/shared/lib/dates'

export function addDays(day: string, n: number): string {
  const d = parseDay(day)
  d.setDate(d.getDate() + n)
  return toDay(d)
}

/** O servidor aceita marcar até 31 dias atrás e nunca o futuro. */
export const MAX_BACK_DAYS = 31

/** Os dias que ainda dá para marcar, do mais recente (ontem) ao mais antigo. */
export function pastDays(today: string): string[] {
  return Array.from({ length: MAX_BACK_DAYS }, (_, i) => addDays(today, -(i + 1)))
}

export const inMarkWindow = (day: string, today: string) => day <= today && day >= addDays(today, -MAX_BACK_DAYS)
