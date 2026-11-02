/** Cidade com o nome em cada idioma e o fuso IANA que o backend aceita em `PATCH /v1/me`. */
export type City = { id: string; pt: string; en: string; tz: string }

const c = (id: string, pt: string, tz: string, en = pt): City => ({ id, pt, en, tz })

/**
 * Lista curta e embutida: capitais brasileiras e algumas cidades do exterior onde moram brasileiros.
 * Cidade do mesmo fuso aparece mais de uma vez de propósito: a pessoa escolhe a sua, não o fuso.
 */
export const CITIES: City[] = [
  c('sao-paulo', 'São Paulo', 'America/Sao_Paulo'),
  c('rio-de-janeiro', 'Rio de Janeiro', 'America/Sao_Paulo'),
  c('brasilia', 'Brasília', 'America/Sao_Paulo'),
  c('belo-horizonte', 'Belo Horizonte', 'America/Sao_Paulo'),
  c('curitiba', 'Curitiba', 'America/Sao_Paulo'),
  c('florianopolis', 'Florianópolis', 'America/Sao_Paulo'),
  c('porto-alegre', 'Porto Alegre', 'America/Sao_Paulo'),
  c('vitoria', 'Vitória', 'America/Sao_Paulo'),
  c('goiania', 'Goiânia', 'America/Sao_Paulo'),
  c('salvador', 'Salvador', 'America/Bahia'),
  c('recife', 'Recife', 'America/Recife'),
  c('fortaleza', 'Fortaleza', 'America/Fortaleza'),
  c('natal', 'Natal', 'America/Fortaleza'),
  c('joao-pessoa', 'João Pessoa', 'America/Fortaleza'),
  c('sao-luis', 'São Luís', 'America/Fortaleza'),
  c('teresina', 'Teresina', 'America/Fortaleza'),
  c('maceio', 'Maceió', 'America/Maceio'),
  c('aracaju', 'Aracaju', 'America/Maceio'),
  c('palmas', 'Palmas', 'America/Araguaina'),
  c('belem', 'Belém', 'America/Belem'),
  c('macapa', 'Macapá', 'America/Belem'),
  c('cuiaba', 'Cuiabá', 'America/Cuiaba'),
  c('campo-grande', 'Campo Grande', 'America/Campo_Grande'),
  c('manaus', 'Manaus', 'America/Manaus'),
  c('porto-velho', 'Porto Velho', 'America/Porto_Velho'),
  c('boa-vista', 'Boa Vista', 'America/Boa_Vista'),
  c('rio-branco', 'Rio Branco', 'America/Rio_Branco'),
  c('fernando-de-noronha', 'Fernando de Noronha', 'America/Noronha'),
  c('lisboa', 'Lisboa', 'Europe/Lisbon', 'Lisbon'),
  c('porto', 'Porto', 'Europe/Lisbon', 'Porto'),
  c('madri', 'Madri', 'Europe/Madrid', 'Madrid'),
  c('paris', 'Paris', 'Europe/Paris'),
  c('londres', 'Londres', 'Europe/London', 'London'),
  c('dublin', 'Dublin', 'Europe/Dublin'),
  c('berlim', 'Berlim', 'Europe/Berlin', 'Berlin'),
  c('nova-york', 'Nova York', 'America/New_York', 'New York'),
  c('miami', 'Miami', 'America/New_York'),
  c('boston', 'Boston', 'America/New_York'),
  c('los-angeles', 'Los Angeles', 'America/Los_Angeles'),
  c('toronto', 'Toronto', 'America/Toronto'),
  c('buenos-aires', 'Buenos Aires', 'America/Argentina/Buenos_Aires'),
  c('montevideu', 'Montevidéu', 'America/Montevideo', 'Montevideo'),
  c('toquio', 'Tóquio', 'Asia/Tokyo', 'Tokyo'),
  c('sydney', 'Sydney', 'Australia/Sydney'),
]

export const cityName = (city: City, lang: string) => (lang.startsWith('en') ? city.en : city.pt)

/** A cidade que corresponde a um fuso: a primeira da lista com esse fuso. */
export const cityForZone = (tz: string) => CITIES.find((city) => city.tz === tz)

/** O fuso do aparelho, se o navegador souber dizer. */
export function deviceZone(): string | undefined {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || undefined
  } catch {
    return undefined
  }
}
