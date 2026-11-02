/**
 * Datas do servidor são "AAAA-MM-DD" (sem fuso) e meses são "AAAA-MM".
 * Nunca use `new Date("2026-11-15")`: é UTC e pode mostrar o dia anterior.
 */

export function parseDay(day: string): Date {
  const [y, m, d] = day.split('-').map(Number)
  return new Date(y, m - 1, d)
}

export function toDay(date: Date): string {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${p(date.getMonth() + 1)}-${p(date.getDate())}`
}

/** O dia de hoje no fuso da pessoa. */
export function todayIn(timeZone?: string, now = new Date()): string {
  const format = (tz?: string) => new Intl.DateTimeFormat('en-CA', { timeZone: tz, year: 'numeric', month: '2-digit', day: '2-digit' }).format(now)
  try {
    return format(timeZone || undefined)
  } catch {
    return format() // fuso vazio ou desconhecido: cai no do aparelho em vez de quebrar a tela
  }
}

export const monthOf = (day: string) => day.slice(0, 7)

export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number)
  const d = new Date(y, m - 1 + delta, 1)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
}

export function formatDay(day: string, lang: string, opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' }) {
  return new Intl.DateTimeFormat(lang, opts).format(parseDay(day))
}

export function formatMonth(month: string, lang: string) {
  return new Intl.DateTimeFormat(lang, { month: 'long', year: 'numeric' }).format(parseDay(`${month}-01`))
}

/** Dias de diferença entre dois "AAAA-MM-DD" (b - a). */
export function daysBetween(a: string, b: string): number {
  return Math.round((parseDay(b).getTime() - parseDay(a).getTime()) / 86_400_000)
}
