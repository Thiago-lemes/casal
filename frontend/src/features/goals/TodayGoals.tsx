import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Button, Card } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { todayIn } from '@/shared/lib/dates'
import type { Goal } from './api'
import { useCheckinActions, useGoals, useModuleGuard } from './hooks'
import { goalsForToday } from './today'
import type { Me } from './types'
import styles from './goals.module.css'

function Row({ goal }: { goal: Goal }) {
  const { t } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const { mark } = useCheckinActions(goal.id)
  return (
    <li className={styles.item}>
      <span className={styles.itemMain}>
        <Link to={`/goals/${goal.id}`} className={styles.itemTitle}>
          {goal.title}
        </Link>
        {mark.error != null && (
          <span className={styles.error} role="alert">
            {errorMessage(tc, mark.error)}
          </span>
        )}
      </span>
      <Button loading={mark.isPending} onClick={() => mark.mutate(undefined)} aria-label={t('today.markFor', { title: goal.title })}>
        {t('today.mark')}
      </Button>
    </li>
  )
}

/**
 * "Hábitos de hoje" e "Metas do casal": o que ainda dá para marcar hoje. Só a tela Hoje o monta, com o módulo ligado.
 * Sem contador de pendentes por pessoa e sem texto de cobrança: some quando não há nada a marcar.
 */
export function TodayGoals({ me }: { me: Me }) {
  const { t } = useTranslation('goals')
  const list = useGoals({ status: 'active' })
  useModuleGuard(list.error)
  const today = todayIn(me.timezone)
  const { habits, shared } = useMemo(() => goalsForToday(list.data ?? [], me.id, today), [list.data, me.id, today])

  return (
    <>
      {habits.length > 0 && (
        <Card title={t('today.habits')} aside={<Link to="/goals">{t('today.seeAll')}</Link>}>
          <ul className={styles.list}>
            {habits.map((g) => (
              <Row key={g.id} goal={g} />
            ))}
          </ul>
        </Card>
      )}
      {shared.length > 0 && (
        <Card title={t('today.shared')} aside={<Link to="/goals">{t('today.seeAll')}</Link>}>
          <ul className={styles.list}>
            {shared.map((g) => (
              <Row key={g.id} goal={g} />
            ))}
          </ul>
        </Card>
      )}
    </>
  )
}
