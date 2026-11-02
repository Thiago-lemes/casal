import { describe, expect, it } from 'vitest'
import { CITIES, cityForZone, cityName } from './cities'

describe('CITIES', () => {
  it('todo fuso é um nome IANA que o navegador conhece', () => {
    for (const city of CITIES) {
      expect(() => new Intl.DateTimeFormat('pt-BR', { timeZone: city.tz }), `${city.id}: ${city.tz}`).not.toThrow()
    }
  })

  it('os ids não se repetem e não há fuso inventado como "Local"', () => {
    const ids = CITIES.map((c) => c.id)
    expect(new Set(ids).size).toBe(ids.length)
    expect(CITIES.map((c) => c.tz)).not.toContain('Local')
  })

  it('o nome muda com o idioma', () => {
    const ny = CITIES.find((c) => c.id === 'nova-york')!
    expect(cityName(ny, 'pt-BR')).toBe('Nova York')
    expect(cityName(ny, 'en')).toBe('New York')
  })

  it('achar a cidade pelo fuso devolve a primeira da lista (São Paulo para o fuso de Brasília)', () => {
    expect(cityForZone('America/Sao_Paulo')?.id).toBe('sao-paulo')
    expect(cityForZone('Mars/Olympus')).toBeUndefined()
  })
})
