/** Formata hora e data no idioma da interface e no fuso do perfil. */
export function formatTime(iso: string, lang: string, timeZone?: string): string {
  return new Intl.DateTimeFormat(lang, { hour: '2-digit', minute: '2-digit', timeZone }).format(new Date(iso))
}

export function formatLongDate(date: Date, lang: string, timeZone?: string) {
  const weekday = new Intl.DateTimeFormat(lang, { weekday: 'long', timeZone }).format(date)
  const day = new Intl.DateTimeFormat(lang, { day: 'numeric', timeZone }).format(date)
  const month = new Intl.DateTimeFormat(lang, { month: 'long', timeZone }).format(date)
  return { weekday, day, month }
}

export function isPast(iso: string, now = new Date()): boolean {
  return new Date(iso).getTime() < now.getTime()
}

/** Atalhos de horário do compositor, calculados no fuso do aparelho. */
export function quickTimes(now = new Date()) {
  const inOneHour = new Date(now.getTime() + 60 * 60 * 1000)

  const tonight = new Date(now)
  tonight.setHours(20, 0, 0, 0)
  if (tonight.getTime() <= now.getTime()) tonight.setDate(tonight.getDate() + 1)

  const tomorrowMorning = new Date(now)
  tomorrowMorning.setDate(tomorrowMorning.getDate() + 1)
  tomorrowMorning.setHours(8, 0, 0, 0)

  return { inOneHour, tonight, tomorrowMorning }
}

export function minutesUntilTomorrowMorning(now = new Date()): number {
  return Math.max(1, Math.min(1440, Math.round((quickTimes(now).tomorrowMorning.getTime() - now.getTime()) / 60000)))
}

/** Hora separada do AM/PM (em idiomas de 24 h, `period` vem vazio), para dar destaque só à hora. */
export function formatTimeParts(iso: string, lang: string, timeZone?: string): { clock: string; period: string } {
  const parts = new Intl.DateTimeFormat(lang, { hour: 'numeric', minute: '2-digit', timeZone }).formatToParts(new Date(iso))
  const period = parts.find((p) => p.type === 'dayPeriod')?.value ?? ''
  const clock = parts
    .filter((p) => p.type !== 'dayPeriod')
    .map((p) => p.value)
    .join('')
    .trim()
  return { clock: /^\d:/.test(clock) && !period ? `0${clock}` : clock, period }
}
