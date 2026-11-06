import { useEffect, useRef, useState } from 'react'
import type { CSSProperties } from 'react'
import styles from './Mascot.module.css'

export type MascotMood = 'idle' | 'hello' | 'cheer' | 'party'

type Props = {
  /** `idle` pisca de vez em quando; `hello` entra caindo; `cheer` pula uma vez; `party` pula sem parar. */
  mood?: MascotMood
  /** Para onde os olhos olham, de -1 a 1 em cada eixo (x para a direita, y para baixo). */
  gaze?: { x: number; y: number }
  /** Sozinha: entra caindo, cumprimenta de tempos em tempos e pula quando tocada. Ignora `mood`. */
  playful?: boolean
  /** `onDark` é para fundo escuro fixo (a faixa azul), com as cores do tema escuro nos dois temas. */
  tone?: 'brand' | 'onDark'
  className?: string
}

const R = 20
const CY = 40
const O1 = 64
const O2 = 108

const half = (cx: number, side: 'l' | 'r') => `M${cx} ${CY - R} A${R} ${R} 0 0 ${side === 'l' ? 0 : 1} ${cx} ${CY + R} Z`

/** Olhos e sorriso; o segundo "o" pisca com o olho direito, como na logo. */
function Face({ cx, wink = false }: { cx: number; wink?: boolean }) {
  return (
    <>
      <g className={styles.gaze}>
        <circle className={styles.eye} cx={cx - 7} cy={34} r={3} />
        {wink ? (
          <path className={styles.wink} d={`M${cx + 3} 35 Q${cx + 7} 30.5 ${cx + 11} 35`} />
        ) : (
          <circle className={styles.eye} cx={cx + 7} cy={34} r={3} />
        )}
      </g>
      <path className={styles.mouth} d={`M${cx - 9} 44.5 Q${cx} 55 ${cx + 9} 44.5`} />
    </>
  )
}

/**
 * A marca (os dois "o" com rosto) viva: olha para onde se pede e reage. Só desenho e movimento:
 * não é lida por leitores de tela e quem a usa não depende dela para nada.
 */
export function Mascot({ mood = 'idle', gaze, playful = false, tone = 'brand', className }: Props) {
  const [auto, setAuto] = useState<MascotMood>('hello')
  const pokeTimer = useRef<number | undefined>(undefined)

  useEffect(() => {
    if (!playful) return
    let reset: number | undefined
    const settle = window.setTimeout(() => setAuto('idle'), 1200)
    const wave = window.setInterval(() => {
      setAuto('cheer')
      reset = window.setTimeout(() => setAuto('idle'), 900)
    }, 6000)
    return () => {
      window.clearTimeout(settle)
      window.clearInterval(wave)
      window.clearTimeout(reset)
    }
  }, [playful])

  const current = playful ? auto : mood
  const style = { '--gx': (gaze?.x ?? 0) * 1.8, '--gy': (gaze?.y ?? 0) * 1.6 } as CSSProperties

  // um toque novo recomeça a contagem; ao sair da tela, nenhum timer fica para trás
  useEffect(() => () => window.clearTimeout(pokeTimer.current), [])

  function poke() {
    if (!playful) return
    setAuto('cheer')
    window.clearTimeout(pokeTimer.current)
    pokeTimer.current = window.setTimeout(() => setAuto('idle'), 900)
  }

  return (
    <svg
      className={[styles.mascot, tone === 'onDark' ? styles.onDark : '', className].filter(Boolean).join(' ')}
      data-mood={current}
      viewBox="40 8 92 56"
      aria-hidden="true"
      focusable="false"
      style={style}
      onClick={poke}
    >
      <g className={`${styles.o} ${styles.o1}`}>
        <path className={styles.a} d={half(O1, 'l')} />
        <path className={styles.b} d={half(O1, 'r')} />
        <Face cx={O1} />
      </g>
      <g className={`${styles.o} ${styles.o2}`}>
        <path className={styles.b} d={half(O2, 'l')} />
        <path className={styles.a} d={half(O2, 'r')} />
        <Face cx={O2} wink />
      </g>
    </svg>
  )
}
