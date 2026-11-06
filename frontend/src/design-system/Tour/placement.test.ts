import { describe, expect, it } from 'vitest'
import { GAP, MARGIN, MASCOT_ROOM, placeBalloon, placeMascot } from './placement'

const pop = { width: 320, height: 200 }
const VW = 390
const VH = 800

describe('placeBalloon', () => {
  it('sem alvo, fica no centro', () => {
    const p = placeBalloon(null, pop, VW, VH)
    expect(p.side).toBe('center')
    expect(p.left).toBe((VW - pop.width) / 2)
    expect(p.top).toBe((VH - pop.height) / 2)
    expect(p.arrow).toBeUndefined()
  })

  it('prefere embaixo do alvo, com a seta apontando para ele', () => {
    const hole = { top: 100, left: 300, width: 60, height: 40 }
    const p = placeBalloon(hole, pop, VW, VH)
    expect(p.side).toBe('below')
    expect(p.top).toBe(100 + 40 + GAP)
    expect(p.left).toBe(VW - pop.width - MARGIN) // não passa da borda da tela
    expect(p.arrow).toBeCloseTo(330 - p.left)
  })

  it('sem lugar embaixo (alvo na barra de baixo), vai para cima', () => {
    const hole = { top: 740, left: 0, width: VW, height: 56 }
    const p = placeBalloon(hole, pop, VW, VH)
    expect(p.side).toBe('above')
    expect(p.top).toBe(740 - GAP - pop.height)
  })

  it('alvo alto (o trilho do desktop) ganha o balão ao lado', () => {
    const hole = { top: 0, left: 0, width: 240, height: 800 }
    const p = placeBalloon(hole, pop, 1280, 800)
    expect(p.side).toBe('right')
    expect(p.left).toBe(240 + GAP)
    expect(p.top).toBeGreaterThanOrEqual(MASCOT_ROOM)
  })

  it('se não cabe em lugar nenhum, fica embaixo da tela', () => {
    const hole = { top: 0, left: 0, width: VW, height: VH }
    const p = placeBalloon(hole, pop, VW, VH)
    expect(p.side).toBe('center')
    expect(p.top).toBe(VH - pop.height - MARGIN)
  })

  it('a seta nunca encosta nos cantos do balão', () => {
    const p = placeBalloon({ top: 100, left: 0, width: 10, height: 10 }, pop, VW, VH)
    expect(p.arrow).toBeGreaterThanOrEqual(24)
  })
})

describe('placeMascot', () => {
  it('senta do lado oposto ao da seta e olha para o alvo', () => {
    const hole = { top: 100, left: 20, width: 40, height: 40 }
    const p = placeBalloon(hole, pop, VW, VH)
    const m = placeMascot(p, pop, 96, hole)
    expect(p.arrow).toBeLessThan(pop.width / 2)
    expect(m.left).toBe(p.left + pop.width - 96 - 12)
    expect(m.top).toBe(p.top - 50)
    expect(m.gaze.y).toBeLessThan(0) // o alvo está acima da logo
  })

  it('sem alvo, olha para a frente', () => {
    const m = placeMascot(placeBalloon(null, pop, VW, VH), pop, 96, null)
    expect(m.gaze).toEqual({ x: 0, y: 0 })
  })
})
