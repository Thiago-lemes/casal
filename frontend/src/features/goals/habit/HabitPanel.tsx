import { useTranslation } from 'react-i18next'
import { Card } from '@/design-system'
import { formatDay } from '@/shared/lib/dates'
import type { Goal } from '../api'
import { goalActions } from '../permissions'
import type { Ctx } from '../types'
import { maskToDays } from '../weekdays'
import { HistoryList } from './HistoryList'
import { MarkControls } from './MarkControls'
import { minToTime } from '../create/form'
import styles from '../goals.module.css'

/** Hábito: o período corrente, o botão de marcar e o histórico em fatos. Só o dono marca. */
export function HabitPanel({ goal, ctx }: { goal: Goal; ctx: Ctx }) {
  const { t, i18n } = useTranslation('goals')
  const habit = goal.habit
  if (!habit) return null
  const period = goal.progress.period
  const days = maskToDays(habit.weekdays)
  // 2 a 8 de nov/2026 é segunda a domingo: rótulos dos dias combinados sem passar por fuso
  const dayNames = days.map((d) => formatDay(`2026-11-0${d + 1}`, i18n.language, { weekday: 'short' }))
  const view = goal.access === 'view'

  return (
    <>
      <Card title={t('habit.title')}>
        <div className={styles.stack}>
          {period && (
            <p className={styles.hint}>
              {t(habit.period === 'week' ? 'habit.currentWeek' : 'habit.currentMonth', {
                from: formatDay(period.start, i18n.language),
                to: formatDay(period.end, i18n.language),
              })}
            </p>
          )}
          <p className={styles.hint}>{days.length > 0 ? t('habit.agreedDays', { days: dayNames.join(', ') }) : t('habit.anyDay')}</p>
          {!view && (
            <p className={styles.hint}>
              {habit.remind_enabled ? t('habit.reminderOn', { time: minToTime(habit.remind_min) }) : t('habit.reminderOff')}
              {goal.status === 'paused' && ` ${t('habit.pausedNote')}`}
            </p>
          )}
          {goalActions(goal).mark && <MarkControls goal={goal} me={ctx.me} />}
        </div>
      </Card>
      {!view && (
        <Card title={t('history.title')}>
          <HistoryList goal={goal} ctx={ctx} />
        </Card>
      )}
    </>
  )
}
