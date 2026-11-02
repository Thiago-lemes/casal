import type { ReactNode } from 'react'
import styles from './Card.module.css'

/** Cartão do painel: um título e uma pergunta respondida. Nunca aninhado em outro cartão. */
export function Card({
  title,
  aside,
  className,
  children,
}: {
  title: string
  aside?: ReactNode
  className?: string
  children: ReactNode
}) {
  return (
    <section className={[styles.card, className].filter(Boolean).join(' ')}>
      <header className={styles.header}>
        <h2 className={styles.title}>{title}</h2>
        {aside}
      </header>
      {children}
    </section>
  )
}
