/** Fuso IANA com o nome em cada idioma; é o que o backend aceita em `PATCH /v1/me`. */
export type Zone = { tz: string; pt: string; en: string }

const z = (tz: string, pt: string, en = pt): Zone => ({ tz, pt, en })

/**
 * Lista curta e embutida: os fusos do Brasil e os do exterior onde moram brasileiros.
 * No Brasil só há quatro deslocamentos hoje (sem horário de verão), então cada um aparece uma vez.
 * Qualquer outro fuso vale pelo campo "Outro fuso".
 */
export const ZONES: Zone[] = [
  z('America/Sao_Paulo', 'Brasília', 'Brasília'),
  z('America/Manaus', 'Manaus, Cuiabá, Campo Grande', 'Manaus, Cuiabá, Campo Grande'),
  z('America/Rio_Branco', 'Acre', 'Acre'),
  z('America/Noronha', 'Fernando de Noronha', 'Fernando de Noronha'),
  z('Europe/Lisbon', 'Lisboa', 'Lisbon'),
  z('Europe/Madrid', 'Madri', 'Madrid'),
  z('Europe/Paris', 'Paris'),
  z('Europe/London', 'Londres', 'London'),
  z('Europe/Berlin', 'Berlim', 'Berlin'),
  z('America/New_York', 'Nova York', 'New York'),
  z('America/Chicago', 'Chicago'),
  z('America/Los_Angeles', 'Los Angeles'),
  z('America/Toronto', 'Toronto'),
  z('America/Argentina/Buenos_Aires', 'Buenos Aires'),
  z('America/Montevideo', 'Montevidéu', 'Montevideo'),
  z('America/Bogota', 'Bogotá', 'Bogotá'),
  z('Asia/Dubai', 'Dubai'),
  z('Asia/Tokyo', 'Tóquio', 'Tokyo'),
  z('Australia/Sydney', 'Sydney'),
]

/**
 * Fusos do Brasil que a lista curta não traz, ligados ao que tem o mesmo deslocamento (sem horário de verão
 * desde 2019). Quem já tinha um deles salvo vê o nome da lista, não o nome IANA.
 */
const ALIAS: Record<string, string> = {
  'America/Bahia': 'America/Sao_Paulo',
  'America/Recife': 'America/Sao_Paulo',
  'America/Fortaleza': 'America/Sao_Paulo',
  'America/Maceio': 'America/Sao_Paulo',
  'America/Belem': 'America/Sao_Paulo',
  'America/Araguaina': 'America/Sao_Paulo',
  'America/Santarem': 'America/Sao_Paulo',
  'America/Cuiaba': 'America/Manaus',
  'America/Campo_Grande': 'America/Manaus',
  'America/Porto_Velho': 'America/Manaus',
  'America/Boa_Vista': 'America/Manaus',
  'America/Eirunepe': 'America/Rio_Branco',
}

/** O fuso da lista curta que representa este (ele mesmo, ou o equivalente no Brasil). */
export const listedZone = (tz: string) => ALIAS[tz] ?? tz

/** Nomes do banco IANA ("Região/Cidade" ou "UTC"); "Local" e vazio não valem, como no backend. */
const IANA = /^(UTC|[A-Za-z_]+(\/[A-Za-z0-9_+-]+)+)$/

/**
 * O nome do fuso como o backend o lê (ele diferencia maiúsculas): o do banco do navegador quando existe,
 * senão o que o `Intl` devolve. `undefined` se o fuso não existe.
 */
export function canonicalZone(tz: string): string | undefined {
  if (!IANA.test(tz)) return undefined
  const lower = tz.toLowerCase()
  const listed = allZones().find((name) => name.toLowerCase() === lower)
  if (listed) return listed
  try {
    return new Intl.DateTimeFormat('pt-BR', { timeZone: tz }).resolvedOptions().timeZone
  } catch {
    return undefined
  }
}

export const isValidZone = (tz: string) => canonicalZone(tz) !== undefined

export const findZone = (tz: string) => ZONES.find((zone) => zone.tz === listedZone(tz))

let cachedZones: string[] | undefined

/** Todos os fusos que o navegador conhece, para a busca de "Outro fuso" (calculado uma vez). */
export function allZones(): string[] {
  if (cachedZones) return cachedZones
  try {
    const list = (Intl as unknown as { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf?.('timeZone')
    cachedZones = list ?? []
  } catch {
    cachedZones = []
  }
  return cachedZones
}

/** "GMT−3", "GMT+5:30" ou "GMT"; calculado na data dada, então respeita o horário de verão. */
export function zoneOffset(tz: string, at: Date = new Date()): string {
  try {
    const name = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'longOffset' })
      .formatToParts(at)
      .find((p) => p.type === 'timeZoneName')?.value
    const m = /^GMT(?:([+-])(\d{2}):(\d{2}))?$/.exec(name ?? '')
    if (!m) return ''
    if (!m[1]) return 'GMT'
    const hours = String(Number(m[2]))
    const minutes = m[3] === '00' ? '' : `:${m[3]}`
    return `GMT${m[1] === '-' ? '−' : '+'}${hours}${minutes}`
  } catch {
    return ''
  }
}

/** "Brasília (GMT−3)"; um fuso fora da lista aparece pelo nome IANA, também com o deslocamento. */
export function zoneLabel(tz: string, lang: string, at?: Date): string {
  const known = findZone(tz)
  const name = known ? (lang.startsWith('en') ? known.en : known.pt) : tz
  const offset = zoneOffset(tz, at)
  return offset ? `${name} (${offset})` : name
}

/** O fuso do aparelho, se o navegador souber dizer. */
export function deviceZone(): string | undefined {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || undefined
  } catch {
    return undefined
  }
}
