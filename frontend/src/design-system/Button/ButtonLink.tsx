import type { AnchorHTMLAttributes } from 'react'
import styles from './Button.module.css'

type Props = AnchorHTMLAttributes<HTMLAnchorElement> & { variant?: 'primary' | 'secondary' | 'quiet' | 'danger' }

/** Um link com a cara do Button (por exemplo, para baixar um arquivo): um só elemento interativo, sem botão dentro de link. */
export function ButtonLink({ variant = 'secondary', className, children, ...rest }: Props) {
  return (
    <a {...rest} className={[styles.button, styles[variant], className].filter(Boolean).join(' ')}>
      {children}
    </a>
  )
}
