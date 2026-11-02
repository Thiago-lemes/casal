import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Card } from '@/design-system'
import { formatTime } from '@/shared/lib/time'
import styles from './TodayTasks.module.css'

type Item = { id: string; title: string; due_at?: string; late: boolean }

/** As tarefas que pedem o dia de hoje; concluir é um toque. */
export function TodayTasks({
  items,
  timezone,
  onComplete,
}: {
  items: Item[]
  timezone: string
  onComplete: (id: string) => void
}) {
  const { t, i18n } = useTranslation('today')

  return (
    <Card title={t('tasks.title')} aside={<Link to="/tasks">{t('tasks.all')}</Link>}>
      {items.length === 0 ? (
        <p className={styles.empty}>{t('tasks.empty')}</p>
      ) : (
        <ul>
          {items.map((it) => (
            <li key={it.id} className={styles.row}>
              <button type="button" className={styles.check} onClick={() => onComplete(it.id)} aria-label={t('tasks.complete', { title: it.title })}>
                <Check size={18} aria-hidden="true" />
              </button>
              <span className={styles.title}>{it.title}</span>
              {it.due_at && (
                <span className={styles.due} data-late={it.late || undefined}>
                  {it.late ? `${t('tasks.late')} · ` : ''}
                  {formatTime(it.due_at, i18n.language, timezone)}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
