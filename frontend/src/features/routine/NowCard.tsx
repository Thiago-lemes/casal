import { useTranslation } from 'react-i18next'
import { Avatar, Card, Skeleton } from '@/design-system'
import type { Presence } from './api'
import { formatClock } from './dates'
import { toneOf } from './types'
import styles from './NowCard.module.css'

type Props = { people?: Presence[]; pending: boolean; meId: string; timezone: string }

/** "Quem está onde agora": uma linha por pessoa; sem nada marcado (ou nada visível), a pessoa está livre. */
export function NowCard({ people, pending, meId, timezone }: Props) {
  const { t, i18n } = useTranslation('routine')

  return (
    <Card title={t('now.title')}>
      {pending && <Skeleton height="3rem" />}
      {people && (
        <ul className={styles.list}>
          {people.map((p) => (
            <li key={p.user_id} className={styles.person}>
              {/* a cor da pessoa acompanha a das linhas do dia e da semana */}
              <span className={styles.tone} data-tone={toneOf(p.user_id, meId)}>
                <Avatar name={p.name} />
              </span>
              <div>
                <p className={styles.name}>{p.user_id === meId ? t('day.you') : p.name}</p>
                <p className={styles.what}>
                  {p.now === null
                    ? t('now.free')
                    : p.now.busy
                      ? t('now.busy', { until: formatClock(p.now.end_at, i18n.language, timezone) })
                      : t('now.doing', {
                          title: p.now.title,
                          until: formatClock(p.now.end_at, i18n.language, timezone),
                        })}
                  {p.now && !p.now.busy && p.now.location ? ` · ${p.now.location}` : ''}
                </p>
              </div>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
