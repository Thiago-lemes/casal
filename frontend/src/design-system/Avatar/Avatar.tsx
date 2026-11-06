import styles from './Avatar.module.css'

/** Círculo com as iniciais. Sem foto por enquanto; a foto opcional do produto entra aqui depois. */
export function Avatar({ name, className }: { name?: string | null; className?: string }) {
  const initials = (name ?? '')
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join('')

  return (
    <span className={[styles.avatar, className].filter(Boolean).join(' ')} aria-hidden="true">
      {initials || '?'}
    </span>
  )
}
