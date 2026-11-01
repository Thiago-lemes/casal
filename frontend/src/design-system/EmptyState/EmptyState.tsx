import type { ReactNode } from 'react'
import styles from './EmptyState.module.css'

/** Estado vazio ou de erro: diz o que houve e oferece a próxima ação. */
export function EmptyState({ title, body, action }: { title: string; body?: string; action?: ReactNode }) {
  return (
    <div className={styles.empty}>
      <h2 className={styles.title}>{title}</h2>
      {body && <p className={styles.body}>{body}</p>}
      {action}
    </div>
  )
}
