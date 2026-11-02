import { useId } from 'react'
import styles from './Logo.module.css'

type Props = {
  /** `full` = nome inteiro (z, os dois "o" com rosto e g); `mark` = só os dois "o". */
  variant?: 'full' | 'mark'
  /** `onAccent` é a versão para fundo laranja, onde as metades laranja sumiriam; `onDark` é a colorida sobre fundo escuro fixo (nos dois temas). */
  tone?: 'brand' | 'onAccent' | 'onDark'
  className?: string
  /** Nome acessível. Sem ele a logo é decorativa (o nome já está em outro texto). */
  title?: string
}

const R = 20
const CY = 40
/** Centros dos dois "o", na ordem do nome. */
const O1 = 64
const O2 = 108

/** Meio círculo, de cima para baixo; `side` diz qual metade. */
function half(cx: number, side: 'left' | 'right') {
  return `M${cx} ${CY - R} A${R} ${R} 0 0 ${side === 'left' ? 0 : 1} ${cx} ${CY + R} Z`
}

/** Dois olhos e um sorriso em `currentColor`; `wink` troca o olho direito por uma piscadinha. */
function Face({ cx, wink = false }: { cx: number; wink?: boolean }) {
  return (
    <>
      <circle cx={cx - 7} cy={34} r={3} fill="currentColor" />
      {wink ? (
        <path d={`M${cx + 3} 35 Q${cx + 7} 30.5 ${cx + 11} 35`} fill="none" stroke="currentColor" strokeWidth={3.2} strokeLinecap="round" />
      ) : (
        <circle cx={cx + 7} cy={34} r={3} fill="currentColor" />
      )}
      <path d={`M${cx - 9} 44.5 Q${cx} 55 ${cx + 9} 44.5`} fill="none" stroke="currentColor" strokeWidth={3.6} strokeLinecap="round" />
    </>
  )
}

/**
 * Cada "o" é um círculo meio azul, meio laranja (o segundo espelhado). O rosto cruza as duas metades,
 * então é desenhado duas vezes, cada uma recortada na sua metade, com a cor que contrasta com ela.
 */
export function Logo({ variant = 'full', tone = 'brand', className, title }: Props) {
  const id = useId()
  const clip = (cx: number, side: 'l' | 'r') => `${id}-${cx}${side}`
  const classes = [styles.logo, tone === 'onAccent' ? styles.onAccent : '', tone === 'onDark' ? styles.onDark : '', className].filter(Boolean).join(' ')

  /** Um "o": `leftIsA` diz se a metade esquerda é a azul (A) ou a laranja (B). */
  const eyes = (cx: number, leftIsA: boolean, wink = false) => (
    <>
      <g clipPath={`url(#${clip(cx, 'l')})`} className={leftIsA ? styles.faceA : styles.faceB}>
        <Face cx={cx} wink={wink} />
      </g>
      <g clipPath={`url(#${clip(cx, 'r')})`} className={leftIsA ? styles.faceB : styles.faceA}>
        <Face cx={cx} wink={wink} />
      </g>
    </>
  )

  return (
    <svg
      className={classes}
      viewBox={variant === 'full' ? '0 20 178 60' : '44 20 84 40'}
      role={title ? 'img' : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
      focusable="false"
    >
      <defs>
        {[O1, O2].flatMap((cx) => [
          <clipPath key={`${cx}l`} id={clip(cx, 'l')}>
            <rect x={cx - R} y={CY - R} width={R} height={2 * R} />
          </clipPath>,
          <clipPath key={`${cx}r`} id={clip(cx, 'r')}>
            <rect x={cx} y={CY - R} width={R} height={2 * R} />
          </clipPath>,
        ])}
      </defs>

      {variant === 'full' && <polygon className={styles.ink} points="0,20 34,20 34,28 10.5,52 34,52 34,60 0,60 0,52 23.5,28 0,28" />}

      <circle className={styles.a} cx={O1} cy={CY} r={R} />
      <path className={styles.b} d={half(O1, 'right')} />
      <circle className={styles.a} cx={O2} cy={CY} r={R} />
      <path className={styles.b} d={half(O2, 'left')} />
      {eyes(O1, true)}
      {eyes(O2, false, true)}

      {variant === 'full' && (
        <>
          <circle className={styles.inkLine} cx={158} cy={CY} r={16} strokeWidth={8} fill="none" />
          <path className={styles.inkLine} d="M174 20 V62 C174 72 166 76 156 76 H148" strokeWidth={8} fill="none" />
        </>
      )}
    </svg>
  )
}
