/**
 * Datas da rotina. O backend fala em dias AAAA-MM-DD no fuso do perfil e em instantes UTC;
 * aqui tudo é convertido pelo fuso informado (nunca pelo do aparelho).
 */

/** Dia (AAAA-MM-DD) de um instante, visto no fuso. */
export function dayKeyIn(date: Date | string, timeZone: string): string {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(
    typeof date === 'string' ? new Date(date) : date,
  )
  const get = (type: string) => parts.find((p) => p.type === type)!.value
  return `${get('year')}-${get('month')}-${get('day')}`
}

const utc = (key: string) => {
  const [y, m, d] = key.split('-').map(Number)
  return new Date(Date.UTC(y, m - 1, d))
}

const toKey = (d: Date) => d.toISOString().slice(0, 10)

/** Soma dias a uma chave AAAA-MM-DD (sem depender de fuso nem de horário de verão). */
export function addDays(key: string, n: number): string {
  const d = utc(key)
  d.setUTCDate(d.getUTCDate() + n)
  return toKey(d)
}

/** 1 segunda a 7 domingo, como no backend. */
export function weekdayOf(key: string): number {
  return ((utc(key).getUTCDay() + 6) % 7) + 1
}

/** A segunda-feira da semana do dia. */
export function weekStart(key: string): string {
  return addDays(key, -(weekdayOf(key) - 1))
}

/** Os 7 dias da semana (segunda a domingo) do dia. */
export function weekDays(key: string): string[] {
  const start = weekStart(key)
  return Array.from({ length: 7 }, (_, i) => addDays(start, i))
}

/** Minutos desde a meia-noite de um instante, no fuso (0 a 1439). */
export function minutesOfDay(iso: string, timeZone: string): number {
  const parts = new Intl.DateTimeFormat('en-GB', { timeZone, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).formatToParts(new Date(iso))
  const get = (type: string) => Number(parts.find((p) => p.type === type)!.value)
  return get('hour') * 60 + get('minute')
}

/** "HH:MM" para minutos; 1440 vira "24:00". */
export function clock(minutes: number): string {
  return `${String(Math.floor(minutes / 60)).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}`
}

/** "HH:MM" (ou "24:00") para minutos. */
export function parseClock(value: string): number {
  const [h, m] = value.split(':').map(Number)
  return h * 60 + m
}

/** Hora e minuto de um instante, no idioma e no fuso. */
export function formatClock(iso: string, lang: string, timeZone: string): string {
  return new Intl.DateTimeFormat(lang, { timeZone, hour: '2-digit', minute: '2-digit' }).format(new Date(iso))
}

/** Um dia por extenso ("segunda-feira, 2 de novembro"), a partir da chave. */
export function formatDay(key: string, lang: string, options: Intl.DateTimeFormatOptions = { weekday: 'long', day: 'numeric', month: 'long' }): string {
  return new Intl.DateTimeFormat(lang, { ...options, timeZone: 'UTC' }).format(utc(key))
}

/** Duração em português ou inglês curto: 90 vira "1 h 30 min". */
export function duration(minutes: number): string {
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return [h ? `${h} h` : '', m ? `${m} min` : ''].filter(Boolean).join(' ')
}
