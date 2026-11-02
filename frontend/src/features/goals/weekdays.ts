/**
 * `weekdays` do servidor é uma máscara: bit 0 = segunda ... bit 6 = domingo; `0` é "qualquer dia".
 * Na tela os dias são números ISO (1 = segunda ... 7 = domingo), em ordem.
 */

export const ALL_DAYS = [1, 2, 3, 4, 5, 6, 7] as const

export function daysToMask(days: readonly number[]): number {
  return days.reduce((mask, d) => (d >= 1 && d <= 7 ? mask | (1 << (d - 1)) : mask), 0)
}

export function maskToDays(mask: number): number[] {
  return ALL_DAYS.filter((d) => (mask & (1 << (d - 1))) !== 0)
}

/** Dia ISO de um "AAAA-MM-DD" (segunda = 1, domingo = 7), sem passar por fuso. */
export function isoWeekday(day: string): number {
  const [y, m, d] = day.split('-').map(Number)
  const js = new Date(y, m - 1, d).getDay()
  return js === 0 ? 7 : js
}

/** Máscara vazia vale qualquer dia. */
export function allowsDay(mask: number, day: string): boolean {
  return mask === 0 || (mask & (1 << (isoWeekday(day) - 1))) !== 0
}
