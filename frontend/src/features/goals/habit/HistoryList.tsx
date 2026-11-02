import { useTranslation } from 'react-i18next'
import { Button, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { formatDay, todayIn } from '@/shared/lib/dates'
import type { DayMarks, Goal, PeriodSummary } from '../api'
import { inMarkWindow } from '../days'
import { useCheckinActions, useHistory } from '../hooks'
import type { Ctx } from '../types'
import { nameOf } from '../types'
import styles from '../goals.module.css'

/** Quem marcou o dia, em ordem fixa por id (sem significado: nada de "quem marcou mais"). */
export function markers(ctx: Ctx, by: string[]): string {
  return [...by].sort().map((id) => nameOf(ctx, id)).filter(Boolean).join(', ')
}

/**
 * Histórico em fatos: uma linha por período (`3 de 3`) e os dias marcados. Sem cor de fracasso, sem sequência,
 * sem "você perdeu": um período sem marcação é só o número.
 */
export function HistoryList({ goal, ctx }: { goal: Goal; ctx: Ctx }) {
  const { t, i18n } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const history = useHistory(goal.id)
  const { undo } = useCheckinActions(goal.id)
  const today = todayIn(ctx.me.timezone)
  const fmt = (d: string) => formatDay(d, i18n.language)

  if (history.isPending) return <Skeleton height="6rem" />
  if (history.isError) {
    return (
      <div className={styles.form}>
        <p className={styles.error} role="alert">
          {errorMessage(tc, history.error)}
        </p>
        <div>
          <Button variant="quiet" onClick={() => void history.refetch()}>
            {tc('actions.tryAgain')}
          </Button>
        </div>
      </div>
    )
  }

  const periods: PeriodSummary[] = [...history.data.periods].sort((a, b) => b.start.localeCompare(a.start))
  const days: DayMarks[] = [...history.data.days].sort((a, b) => b.day.localeCompare(a.day))
  if (periods.length === 0 && days.length === 0) return <p className={styles.hint}>{t('history.empty')}</p>

  return (
    <div className={styles.stack}>
      {periods.length > 0 && (
        <section aria-label={t('history.periods')} className={styles.stack}>
          <h3 className={styles.sectionTitle}>{t('history.periods')}</h3>
          <ul className={styles.list}>
            {periods.map((p) => (
              <li key={p.start} className={styles.item}>
                <span>{t('history.range', { from: fmt(p.start), to: fmt(p.end) })}</span>
                <span className={styles.money}>{t('history.count', { count: p.count, target: p.target })}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {days.length > 0 && (
        <section aria-label={t('history.days')} className={styles.stack}>
          <h3 className={styles.sectionTitle}>{t('history.days')}</h3>
          <ul className={styles.list}>
            {days.map((d) => (
              <li key={d.day} className={styles.item}>
                <span className={styles.itemMain}>
                  <span className={styles.itemTitle}>{formatDay(d.day, i18n.language, { weekday: 'short', day: 'numeric', month: 'short' })}</span>
                  <span className={styles.meta}>
                    {goal.kind === 'shared' && <span>{markers(ctx, d.by)}</span>}
                    {!d.counted && <span>{t(goal.kind === 'shared' ? 'history.waitingOther' : 'history.outside')}</span>}
                  </span>
                </span>
                {d.by.includes(ctx.me.id) && inMarkWindow(d.day, today) && (
                  <Button variant="quiet" loading={undo.isPending && undo.variables === d.day} onClick={() => undo.mutate(d.day)}>
                    {t('mark.undo')}
                  </Button>
                )}
              </li>
            ))}
          </ul>
          {undo.error != null && (
            <p className={styles.error} role="alert">
              {errorMessage(tc, undo.error)}
            </p>
          )}
        </section>
      )}
    </div>
  )
}
