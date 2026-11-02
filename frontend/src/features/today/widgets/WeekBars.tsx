import { useTranslation } from 'react-i18next'
import type { WeekDay } from '@/features/reminders/stats'
import { Card } from '@/design-system'
import styles from './WeekBars.module.css'

/** Baixas dos últimos 7 dias; a barra de hoje é a única em laranja. */
export function WeekBars({ week }: { week: WeekDay[] }) {
  const { t, i18n } = useTranslation('today')
  const max = Math.max(1, ...week.map((d) => d.done))
  const total = week.reduce((n, d) => n + d.done, 0)

  return (
    <Card title={t('week.title')} aside={<p className={styles.total}>{t('week.total', { count: total })}</p>}>
      <ol className={styles.bars} aria-label={t('week.title')}>
        {week.map((d) => {
          const label = new Intl.DateTimeFormat(i18n.language, { weekday: 'narrow' }).format(d.date)
          const full = new Intl.DateTimeFormat(i18n.language, { weekday: 'long' }).format(d.date)
          return (
            <li key={d.key} className={styles.col}>
              <span className={styles.count}>{d.done}</span>
              <span
                className={styles.bar}
                data-today={d.today || undefined}
                style={{ height: `${Math.max(6, (d.done / max) * 100)}%` }}
                role="img"
                aria-label={t('week.day', { day: full, count: d.done })}
              />
              <span className={styles.day} aria-hidden="true">
                {label}
              </span>
            </li>
          )
        })}
      </ol>
    </Card>
  )
}
