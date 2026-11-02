import { useTranslation } from 'react-i18next'
import { Card } from '@/design-system'
import styles from './DayRing.module.css'

const R = 52
const C = 2 * Math.PI * R

/** Quanto do dia já foi dado baixa. Sem nada para hoje, diz isso em vez de mostrar 0%. */
export function DayRing({ progress, done, total }: { progress: number | null; done: number; total: number }) {
  const { t } = useTranslation('today')
  const pct = progress === null ? 0 : Math.round(progress * 100)

  return (
    <Card title={t('ring.title')}>
      <div className={styles.wrap}>
        <svg viewBox="0 0 128 128" className={styles.svg} role="img" aria-label={t('ring.label', { done, total })}>
          <circle cx="64" cy="64" r={R} className={styles.track} />
          <circle
            cx="64"
            cy="64"
            r={R}
            className={styles.value}
            strokeDasharray={C}
            strokeDashoffset={C * (1 - (progress ?? 0))}
            transform="rotate(-90 64 64)"
          />
        </svg>
        <p className={styles.center}>
          {progress === null ? <span className={styles.clear}>{t('ring.clear')}</span> : <>{pct}%</>}
        </p>
      </div>
      {progress !== null && (
        <p className={styles.caption}>{t('ring.caption', { done, total })}</p>
      )}
    </Card>
  )
}
