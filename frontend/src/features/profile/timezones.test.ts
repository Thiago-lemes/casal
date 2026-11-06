import { describe, expect, it } from 'vitest'
import { ZONES, isValidZone, zoneLabel, zoneOffset } from './timezones'

describe('ZONES', () => {
  it('todo fuso é um nome IANA que o navegador conhece, sem repetir', () => {
    for (const zone of ZONES) expect(isValidZone(zone.tz), zone.tz).toBe(true)
    expect(new Set(ZONES.map((z) => z.tz)).size).toBe(ZONES.length)
  })
})

describe('isValidZone', () => {
  it('aceita IANA e UTC; recusa "Local", vazio e fuso inventado', () => {
    expect(isValidZone('America/Lima')).toBe(true)
    expect(isValidZone('UTC')).toBe(true)
    expect(isValidZone('Local')).toBe(false)
    expect(isValidZone('')).toBe(false)
    expect(isValidZone('Mars/Olympus')).toBe(false)
  })
})

describe('zoneOffset / zoneLabel', () => {
  const winter = new Date('2026-01-15T12:00:00Z')
  const summer = new Date('2026-07-15T12:00:00Z')

  it('mostra o deslocamento no formato GMT−3, com minutos só quando existem', () => {
    expect(zoneOffset('America/Sao_Paulo', winter)).toBe('GMT−3')
    expect(zoneOffset('Asia/Kolkata', winter)).toBe('GMT+5:30')
    expect(zoneOffset('UTC', winter)).toBe('GMT')
  })

  it('acompanha o horário de verão', () => {
    expect(zoneOffset('Europe/Lisbon', winter)).toBe('GMT')
    expect(zoneOffset('Europe/Lisbon', summer)).toBe('GMT+1')
  })

  it('o rótulo junta o nome no idioma e o deslocamento', () => {
    expect(zoneLabel('America/Sao_Paulo', 'pt-BR', winter)).toBe('Brasília (GMT−3)')
    expect(zoneLabel('America/New_York', 'pt-BR', winter)).toBe('Nova York (GMT−5)')
    expect(zoneLabel('America/New_York', 'en', winter)).toBe('New York (GMT−5)')
  })

  it('um fuso fora da lista aparece pelo nome IANA', () => {
    expect(zoneLabel('America/Lima', 'pt-BR', winter)).toBe('America/Lima (GMT−5)')
  })
})
