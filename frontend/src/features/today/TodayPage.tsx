import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Button, EmptyState, Skeleton } from '@/design-system'
import { useMe } from '@/features/auth/hooks'
import { useHousehold } from '@/features/household/hooks'
import { isPending } from '@/features/reminders/api'
import { Composer } from '@/features/reminders/Composer'
import { ReminderLabel } from '@/features/reminders/ReminderLabel'
import { PushPrompt } from '@/features/push/PushPrompt'
import { tasksForToday } from '@/features/tasks/today'
import { useTaskActions, useTasks } from '@/features/tasks/hooks'
import { useReminderActions, useReminders } from '@/features/reminders/hooks'
import { busyDays, dayStats, weekSeries } from '@/features/reminders/stats'
import { formatLongDate } from '@/shared/lib/time'
import { TodayTasks } from './widgets/TodayTasks'
import { DayRing } from './widgets/DayRing'
import { MonthCalendar } from './widgets/MonthCalendar'
import { WeekBars } from './widgets/WeekBars'
import styles from './TodayPage.module.css'

export function TodayPage() {
  const { t, i18n } = useTranslation('today')
  const { t: tc } = useTranslation('common')
  const me = useMe().data
  const household = useHousehold().data
  const reminders = useReminders(Boolean(household))
  const actions = useReminderActions()
  const openTasks = useTasks('open', Boolean(household))
  const taskActions = useTaskActions()
  const [composing, setComposing] = useState(false)

  const all = reminders.data
  const pending = useMemo(
    () => (all ?? []).filter(isPending).sort((a, b) => a.remind_at.localeCompare(b.remind_at)),
    [all],
  )
  const stats = useMemo(() => dayStats(all ?? []), [all])
  const week = useMemo(() => weekSeries(all ?? []), [all])
  const busy = useMemo(() => busyDays(all ?? []), [all])

  const todayTasks = useMemo(() => tasksForToday(openTasks.data ?? [], me?.id ?? ''), [openTasks.data, me?.id])

  if (!me || !household) return null

  const partner = household.members.find((m) => m.id !== me.id)
  const { weekday, day, month } = formatLongDate(new Date(), i18n.language, me.timezone)
  const dueTotal = stats.doneToday + stats.dueToday

  return (
    <>
      <PushPrompt />
      <div className={styles.page}>
      <header className={styles.day}>
        <div className={styles.dayTop}>
          <p className={styles.date}>
            <span className={styles.weekday}>{weekday}</span>
            <br />
            {day} {month}
          </p>
          <Button variant="primary" className={styles.cta} onClick={() => setComposing(true)}>
            {t('newReminder')}
          </Button>
        </div>
        <p className={styles.greeting}>{t('greeting', { name: me.name })}</p>
        <dl className={styles.counts}>
          <div>
            <dt>{t('counts.pending')}</dt>
            <dd>{stats.pending}</dd>
          </div>
          <div>
            <dt>{t('counts.overdue')}</dt>
            <dd>{stats.overdue}</dd>
          </div>
          <div>
            <dt>{t('counts.done')}</dt>
            <dd>{stats.doneToday}</dd>
          </div>
        </dl>
      </header>

      <div className={styles.ring}>
        <DayRing progress={stats.progress} done={stats.doneToday} total={dueTotal} />
      </div>

      <section className={styles.list} aria-live="polite" aria-label={t('listTitle')}>
        <div className={styles.listHead}>
          <h2 className={styles.listTitle}>{t('listTitle')}</h2>
          <Link to="/reminders">{t('seeAll')}</Link>
        </div>

        {reminders.isPending && (
          <div className={styles.skeletons}>
            <Skeleton />
            <Skeleton />
          </div>
        )}

        {reminders.isError && (
          <EmptyState
            title={t('loadError')}
            action={<Button onClick={() => void reminders.refetch()}>{tc('actions.tryAgain')}</Button>}
          />
        )}

        {reminders.isSuccess && pending.length === 0 && (
          <EmptyState
            title={t('emptyTitle')}
            body={t('emptyBody')}
            action={
              <Button variant="primary" onClick={() => setComposing(true)}>
                {t('newReminder')}
              </Button>
            }
          />
        )}

        {reminders.isSuccess && pending.length > 0 && (
          <ul>
            {pending.map((r) => (
              <ReminderLabel key={r.id} reminder={r} me={me} partnerName={partner?.name} actions={actions} />
            ))}
          </ul>
        )}
      </section>

      <div className={styles.tasks}>
        <TodayTasks
          timezone={me.timezone}
          items={todayTasks.map((t) => ({
            id: t.id,
            title: t.title,
            due_at: t.due_at,
            late: !!t.due_at && new Date(t.due_at) < new Date(),
          }))}
          onComplete={(id) => taskActions.complete.mutate(id)}
        />
      </div>

      <div className={styles.calendar}>
        <MonthCalendar busy={busy} />
      </div>
      <div className={styles.week}>
        <WeekBars week={week} />
      </div>

      <Composer open={composing} onOpenChange={setComposing} me={me} partner={partner} actions={actions} />
      </div>
    </>
  )
}
