export type Box = { top: number; left: number; width: number; height: number }
export type Side = 'below' | 'above' | 'right' | 'left' | 'center'
export type Placement = {
  side: Side
  top: number
  left: number
  /** onde a seta encosta na borda do balão, em px a partir do canto (ausente quando não há seta) */
  arrow?: number
}

/** Folga entre o balão e a borda da janela. */
export const MARGIN = 12
/** Espaço acima do balão para a logo sentar na borda. */
export const MASCOT_ROOM = 58
/** Distância entre o destaque e o balão. */
export const GAP = 14

const clamp = (n: number, min: number, max: number) => Math.max(min, Math.min(max, n))

/**
 * Onde pôr o balão em relação ao destaque (`hole`, já com a folga). Tenta embaixo e em cima; se o alvo é alto
 * (o trilho do desktop), tenta ao lado primeiro. Sem alvo, ou sem lugar em nenhum lado, vai para o centro.
 */
export function placeBalloon(hole: Box | null, pop: { width: number; height: number }, vw: number, vh: number): Placement {
  const { width: w, height: h } = pop
  const centered: Placement = hole
    ? { side: 'center', top: Math.max(MASCOT_ROOM, vh - h - MARGIN), left: (vw - w) / 2 }
    : { side: 'center', top: Math.max(MASCOT_ROOM, (vh - h) / 2), left: (vw - w) / 2 }
  if (!hole) return centered

  const cx = hole.left + hole.width / 2
  const cy = hole.top + hole.height / 2
  const around = (side: Side): Placement | null => {
    switch (side) {
      case 'below': {
        const top = hole.top + hole.height + GAP
        if (top + h > vh - MARGIN) return null
        const left = clamp(cx - w / 2, MARGIN, Math.max(MARGIN, vw - w - MARGIN))
        return { side, top, left, arrow: clamp(cx - left, 24, w - 24) }
      }
      case 'above': {
        const top = hole.top - GAP - h
        if (top < MASCOT_ROOM) return null
        const left = clamp(cx - w / 2, MARGIN, Math.max(MARGIN, vw - w - MARGIN))
        return { side, top, left, arrow: clamp(cx - left, 24, w - 24) }
      }
      case 'right': {
        const left = hole.left + hole.width + GAP
        if (left + w > vw - MARGIN) return null
        const top = clamp(cy - h / 2, MASCOT_ROOM, Math.max(MASCOT_ROOM, vh - h - MARGIN))
        return { side, top, left, arrow: clamp(cy - top, 24, h - 24) }
      }
      case 'left': {
        const left = hole.left - GAP - w
        if (left < MARGIN) return null
        const top = clamp(cy - h / 2, MASCOT_ROOM, Math.max(MASCOT_ROOM, vh - h - MARGIN))
        return { side, top, left, arrow: clamp(cy - top, 24, h - 24) }
      }
      default:
        return null
    }
  }

  const order: Side[] = hole.height > vh * 0.5 ? ['right', 'left', 'below', 'above'] : ['below', 'above', 'right', 'left']
  for (const side of order) {
    const p = around(side)
    if (p) return p
  }
  return centered
}

/** Onde a logo senta: na borda de cima do balão, do lado oposto ao da seta, e para onde ela olha. */
export function placeMascot(p: Placement, pop: { width: number }, size: number, hole: Box | null) {
  const arrowOnLeft = (p.side === 'below' || p.side === 'above') && p.arrow !== undefined && p.arrow < pop.width / 2
  const left = arrowOnLeft ? p.left + pop.width - size - 12 : p.left + 12
  const top = p.top - 50
  if (!hole) return { left, top, gaze: { x: 0, y: 0 } }
  const dx = hole.left + hole.width / 2 - (left + size / 2)
  const dy = hole.top + hole.height / 2 - (top + 28)
  const len = Math.hypot(dx, dy) || 1
  return { left, top, gaze: { x: dx / len, y: dy / len } }
}
