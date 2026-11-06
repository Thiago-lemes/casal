import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Button } from '../Button/Button'
import { Mascot } from '../Logo/Mascot'
import type { Box } from './placement'
import { placeBalloon, placeMascot } from './placement'
import styles from './Tour.module.css'

export type TourStep = {
  id: string
  /** valor do `data-tour` do elemento a destacar; sem ele (ou se o elemento não está na tela) o balão fica ao centro */
  target?: string
  title: string
  body: string
  /** o que a pessoa faz ali mesmo (botões, formulário); um passo com `content` ou `hint` pede uma ação */
  content?: ReactNode
  /** o alvo recebe o toque (a ação é nele); sem isso o destaque é só para olhar e o toque não passa */
  clickThrough?: boolean
  /** dica curta para quando a ação é no próprio elemento destacado */
  hint?: string
  /** a ação do passo já foi feita: o botão troca de "Pular" para "Continuar" e a dica some (o `content` fica, para mostrar o resultado) */
  done?: boolean
}

export type TourLabels = {
  step: (n: number, total: number) => string
  next: string
  back: string
  skipStep: string
  skipTour: string
  finish: string
}

type Props = {
  steps: TourStep[]
  index: number
  onIndexChange: (index: number) => void
  /** terminou (último botão) */
  onFinish: () => void
  /** fechou antes do fim (Esc ou "Pular o tour") */
  onClose: () => void
  labels: TourLabels
  /** esconde o tour sem perder o passo (ex.: enquanto um diálogo aberto pelo alvo está na tela) */
  paused?: boolean
  /** some 1 a cada conquista: a logo comemora */
  cheer?: number
}

const PAD = 6
const MASCOT_SIZE = 96
const SPARKS = 8
const CONFETTI = 30
const CONFETTI_COLORS = ['var(--color-accent)', 'var(--color-hero)', '#fff6ee', '#6d8bff', '#f98a42']

/** O primeiro elemento com esse `data-tour` que ocupa lugar na tela (a mesma peça pode existir escondida em outro layout). */
function findTarget(name: string): Element | null {
  for (const el of document.querySelectorAll(`[data-tour="${name}"]`)) {
    const r = el.getBoundingClientRect()
    if (r.width > 0 && r.height > 0) return el
  }
  return null
}

const same = (a: Box | null, b: Box | null) =>
  a === b || (a !== null && b !== null && a.top === b.top && a.left === b.left && a.width === b.width && a.height === b.height)

function useTargetBox(name: string | undefined, active: boolean): Box | null {
  const [box, setBox] = useState<Box | null>(null)
  useLayoutEffect(() => {
    if (!active || !name) {
      setBox(null)
      return
    }
    const measure = () => {
      const el = findTarget(name)
      if (!el) return setBox(null)
      const r = el.getBoundingClientRect()
      const next = { top: r.top, left: r.left, width: r.width, height: r.height }
      setBox((prev) => (same(prev, next) ? prev : next))
    }
    findTarget(name)?.scrollIntoView?.({ block: 'nearest' })
    measure()
    let raf = 0
    const schedule = () => {
      cancelAnimationFrame(raf)
      raf = requestAnimationFrame(measure)
    }
    window.addEventListener('resize', schedule)
    window.addEventListener('scroll', schedule, true)
    const el = findTarget(name)
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(schedule)
    if (el) observer?.observe(el)
    const late = window.setTimeout(measure, 300) // a página ainda pode estar se acomodando
    return () => {
      cancelAnimationFrame(raf)
      window.clearTimeout(late)
      window.removeEventListener('resize', schedule)
      window.removeEventListener('scroll', schedule, true)
      observer?.disconnect()
    }
  }, [name, active])
  return box
}

function useViewport() {
  const [vp, setVp] = useState(() => ({ w: window.innerWidth, h: window.innerHeight }))
  useEffect(() => {
    const on = () => setVp({ w: window.innerWidth, h: window.innerHeight })
    window.addEventListener('resize', on)
    return () => window.removeEventListener('resize', on)
  }, [])
  return vp
}

/**
 * Tour com destaques: escurece a tela, deixa livre só o elemento apontado e mostra um balão com a logo
 * como guia (ela olha para o alvo e comemora o que a pessoa faz). Não conhece nenhuma feature: quem usa
 * entrega os passos, e o conteúdo interativo de cada um vem em `content`.
 */
export function Tour({ steps, index, onIndexChange, onFinish, onClose, labels, paused = false, cheer = 0 }: Props) {
  const titleId = useId()
  const step = steps[Math.min(index, steps.length - 1)]
  const at = steps.indexOf(step)
  const last = at === steps.length - 1
  const vp = useViewport()
  const box = useTargetBox(step?.target, !paused && Boolean(step))
  const popRef = useRef<HTMLDivElement>(null)
  const titleRef = useRef<HTMLHeadingElement>(null)
  const [size, setSize] = useState({ width: 320, height: 220 })

  // o balão muda de tamanho quando o conteúdo muda (ex.: o código do convite aparece)
  useLayoutEffect(() => {
    const el = popRef.current
    if (!el) return
    const measure = () => setSize((s) => (s.width === el.offsetWidth && s.height === el.offsetHeight ? s : { width: el.offsetWidth, height: el.offsetHeight }))
    measure()
    if (typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [at, paused])

  // ao trocar de passo o foco vai para o título (leitor de tela anuncia; o teclado segue dali)
  useEffect(() => {
    if (!paused) titleRef.current?.focus({ preventScroll: true })
  }, [at, paused])

  useEffect(() => {
    if (paused) return
    const onKey = (e: KeyboardEvent) => {
      // Esc dentro de um campo (ex.: fechar a lista de cidades) é do campo, não do tour
      if (e.key === 'Escape' && !e.defaultPrevented && !(e.target instanceof Element && e.target.closest('select, input, textarea'))) onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [paused, onClose])

  // a logo comemora quando `cheer` sobe (não na primeira renderização)
  const [cheering, setCheering] = useState(false)
  const [burst, setBurst] = useState(0)
  const first = useRef(true)
  useEffect(() => {
    if (first.current) {
      first.current = false
      return
    }
    setCheering(true)
    setBurst((b) => b + 1)
    const t = window.setTimeout(() => setCheering(false), 900)
    return () => window.clearTimeout(t)
  }, [cheer])

  const confetti = useMemo(
    () =>
      Array.from({ length: CONFETTI }, (_, i) => ({
        left: `${(i * 37) % 100}%`,
        delay: `${(i % 7) * 70}ms`,
        dx: `${((i * 53) % 120) - 60}px`,
        rot: `${((i * 97) % 720) - 360}deg`,
        color: CONFETTI_COLORS[i % CONFETTI_COLORS.length],
      })),
    [],
  )

  if (!step || paused) return null

  const hole: Box | null = box && { top: box.top - PAD, left: box.left - PAD, width: box.width + PAD * 2, height: box.height + PAD * 2 }
  const place = placeBalloon(hole, size, vp.w, vp.h)
  const guide = placeMascot(place, size, MASCOT_SIZE, hole)
  const asksAction = step.content !== undefined || step.hint !== undefined
  const mood = cheering ? 'cheer' : last ? 'party' : 'idle'

  const rect = (top: number, left: number, width: number, height: number) => ({ top, left, width: Math.max(0, width), height: Math.max(0, height) })

  return createPortal(
    <div className={styles.root}>
      {hole ? (
        <>
          <div className={styles.scrim} style={rect(0, 0, vp.w, hole.top)} />
          <div className={styles.scrim} style={rect(hole.top + hole.height, 0, vp.w, vp.h - hole.top - hole.height)} />
          <div className={styles.scrim} style={rect(hole.top, 0, hole.left, hole.height)} />
          <div className={styles.scrim} style={rect(hole.top, hole.left + hole.width, vp.w - hole.left - hole.width, hole.height)} />
          <div className={styles.ring} style={rect(hole.top, hole.left, hole.width, hole.height)} />
          {!step.clickThrough && <div className={styles.shield} style={rect(hole.top, hole.left, hole.width, hole.height)} />}
        </>
      ) : (
        <div className={styles.scrim} style={rect(0, 0, vp.w, vp.h)} />
      )}

      {last && (
        <div className={styles.confetti} aria-hidden="true">
          {confetti.map((c, i) => (
            <span
              key={i}
              style={{ left: c.left, animationDelay: c.delay, background: c.color, ['--dx' as string]: c.dx, ['--rot' as string]: c.rot }}
            />
          ))}
        </div>
      )}

      <div className={styles.guide} style={{ top: guide.top, left: guide.left, width: MASCOT_SIZE }} aria-hidden="true">
        <div key={`hop-${at}`} className={styles.hop}>
          <Mascot mood={mood} gaze={guide.gaze} />
        </div>
        {burst > 0 && (
          <span key={`spark-${burst}`} className={styles.sparks}>
            {Array.from({ length: SPARKS }, (_, i) => {
              const a = (i / SPARKS) * Math.PI * 2
              return <i key={i} style={{ ['--dx' as string]: `${Math.cos(a) * 42}px`, ['--dy' as string]: `${Math.sin(a) * 34 - 8}px` }} />
            })}
          </span>
        )}
      </div>

      <div
        ref={popRef}
        className={styles.balloon}
        role="dialog"
        aria-modal="false"
        aria-labelledby={titleId}
        data-side={place.side}
        style={{ top: place.top, left: place.left, ['--arrow' as string]: place.arrow === undefined ? undefined : `${place.arrow}px` }}
      >
        <p className={styles.kicker}>{labels.step(at + 1, steps.length)}</p>
        <h2 id={titleId} className={styles.title} ref={titleRef} tabIndex={-1}>
          {step.title}
        </h2>
        <p className={styles.body}>{step.body}</p>
        {step.content !== undefined && <div className={styles.action}>{step.content}</div>}
        {step.hint && !step.done && <p className={styles.hint}>{step.hint}</p>}

        <div className={styles.foot}>
          <span className={styles.dots} aria-hidden="true">
            {steps.map((s, i) => (
              <i key={s.id} data-state={i === at ? 'now' : i < at ? 'past' : undefined} />
            ))}
          </span>
          {at > 0 && !last && (
            <Button variant="quiet" onClick={() => onIndexChange(at - 1)}>
              {labels.back}
            </Button>
          )}
          {last ? (
            <Button variant="primary" onClick={onFinish}>
              {labels.finish}
            </Button>
          ) : (
            <Button variant={asksAction && !step.done ? 'secondary' : 'primary'} onClick={() => onIndexChange(at + 1)}>
              {asksAction && !step.done ? labels.skipStep : labels.next}
            </Button>
          )}
        </div>
        {!last && (
          <Button variant="quiet" className={styles.skipAll} onClick={onClose}>
            {labels.skipTour}
          </Button>
        )}
      </div>
    </div>,
    document.body,
  )
}
