import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { formatDay } from '@/shared/lib/dates'
import type { Goal } from './api'
import { GoalProgress } from './GoalProgress'
import { KIND_ICONS } from './kinds'
import type { Person } from './types'
import styles from './goals.module.css'

/**
 * Um cartão da lista. Não destaca, ordena nem compara pessoas: mostra a meta e o progresso que o servidor calculou.
 * Quem só enxerga (`access = 'view'`) vê o selo "Compartilhada por" e nenhuma ação.
 */
export function GoalCard({ goal, people }: { goal: Goal; people: Person[] }) {
  const { t, i18n } = useTranslation('goals')
  const Icon = KIND_ICONS[goal.kind]
  const owner = people.find((p) => p.id === goal.owner_id)

  return (
    <Link to={`/goals/${goal.id}`} className={styles.goalCard} data-paused={goal.status === 'paused' || undefined}>
      <div className={styles.goalHead}>
        <h3 className={styles.goalTitle}>{goal.title}</h3>
        <span className={styles.badges}>
          {goal.status === 'paused' && <span className={styles.badge}>{t('status.paused')}</span>}
          {goal.status === 'done' && <span className={styles.badge} data-tone="ok">{t('status.done')}</span>}
          {goal.status === 'archived' && <span className={styles.badge}>{t('status.archived')}</span>}
          {goal.status === 'active' && goal.progress.suggest_complete && (
            <span className={styles.badge} data-tone="accent">{t('card.readyToComplete')}</span>
          )}
        </span>
      </div>
      <p className={styles.meta}>
        <Icon size={16} aria-hidden="true" />
        <span>{t(`kind.${goal.kind}`)}</span>
        {goal.due_on && <span>· {t('card.due', { date: formatDay(goal.due_on, i18n.language, { day: 'numeric', month: 'short', year: 'numeric' }) })}</span>}
        {goal.access === 'view' && <span className={styles.badge}>{t('card.sharedBy', { name: owner?.name ?? t('card.partner') })}</span>}
      </p>
      <GoalProgress progress={goal.progress} label={goal.title} />
    </Link>
  )
}
