import { useTranslation } from 'react-i18next'
import { Card } from '@/design-system'
import { formatDay, todayIn } from '@/shared/lib/dates'
import type { Goal } from '../api'
import { HistoryList, markers } from '../habit/HistoryList'
import { MarkControls } from '../habit/MarkControls'
import { goalActions } from '../permissions'
import type { Ctx } from '../types'
import styles from '../goals.module.css'

/**
 * Meta do casal: cada um marca a sua. Mostra quem marcou hoje (em ordem fixa por id, sem totais por pessoa)
 * e o progresso do casal. Quando conta só com os dois, um dia com uma marcação aparece como "falta a outra
 * pessoa", sem nome de culpado.
 */
export function SharedPanel({ goal, ctx }: { goal: Goal; ctx: Ctx }) {
  const { t, i18n } = useTranslation('goals')
  const shared = goal.shared
  if (!shared) return null
  const period = goal.progress.period
  const today = todayIn(ctx.me.timezone)
  const todayMarks = goal.recent?.checkins?.find((d) => d.day === today)?.by ?? []
  const pending = goal.progress.pending_days ?? []

  return (
    <>
      <Card title={t('shared.title')}>
        <div className={styles.stack}>
          {period && (
            <p className={styles.hint}>
              {t(shared.period === 'week' ? 'habit.currentWeek' : 'habit.currentMonth', {
                from: formatDay(period.start, i18n.language),
                to: formatDay(period.end, i18n.language),
              })}
            </p>
          )}
          <p className={styles.hint}>{t(shared.requires_both ? 'shared.countsBoth' : 'shared.countsAny')}</p>

          <p role="status">{todayMarks.length > 0 ? t('shared.markedToday', { names: markers(ctx, todayMarks) }) : t('shared.noneToday')}</p>

          {shared.requires_both && pending.length > 0 && (
            <p className={styles.notice}>
              {t('shared.waitingOther', { days: pending.map((d) => formatDay(d, i18n.language)).join(', ') })}
            </p>
          )}

          {goalActions(goal).mark && <MarkControls goal={goal} me={ctx.me} />}
        </div>
      </Card>
      {goal.access !== 'view' && (
        <Card title={t('history.title')}>
          <HistoryList goal={goal} ctx={ctx} />
        </Card>
      )}
    </>
  )
}
