import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, EmptyState, Skeleton } from '@/design-system'
import { useMe } from '@/features/auth/hooks'
import { useHousehold } from '@/features/household/hooks'
import { Composer } from '@/features/reminders/Composer'
import { ReminderLabel } from '@/features/reminders/ReminderLabel'
import { usePendingReminders, useReminderActions } from '@/features/reminders/hooks'
import { formatLongDate } from '@/shared/lib/time'
import styles from './TodayPage.module.css'

export function TodayPage() {
  const { t, i18n } = useTranslation('today')
  const { t: tc } = useTranslation('common')
  const me = useMe().data
  const household = useHousehold().data
  const reminders = usePendingReminders(Boolean(household))
  const actions = useReminderActions()
  const [composing, setComposing] = useState(false)

  if (!me || !household) return null

  const partner = household.members.find((m) => m.id !== me.id)
  const { weekday, day, month } = formatLongDate(new Date(), i18n.language, me.timezone)
  const count = reminders.data?.length ?? 0

  return (
    <>
      <header className={styles.band}>
        <div>
          <p className={styles.date}>
            <span className={styles.weekday}>{weekday}</span>
            <br />
            {day} {month}
          </p>
          <p className={styles.greeting}>
            {t('greeting', { name: me.name })}
            {reminders.isSuccess && count > 0 && ` · ${t('pendingCount', { count })}`}
          </p>
        </div>
        <Button variant="primary" className={styles.cta} onClick={() => setComposing(true)}>
          {t('newReminder')}
        </Button>
      </header>

      <section className={styles.list} aria-live="polite">
        {reminders.isPending && (
          <div className={styles.skeletons}>
            <Skeleton />
            <Skeleton />
          </div>
        )}

        {reminders.isError && (
          <EmptyState
            title={t('loadError')}
            action={
              <Button onClick={() => void reminders.refetch()}>{tc('actions.tryAgain')}</Button>
            }
          />
        )}

        {reminders.isSuccess && count === 0 && (
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

        {reminders.isSuccess && count > 0 && (
          <ul>
            {reminders.data.map((r) => (
              <ReminderLabel key={r.id} reminder={r} me={me} partnerName={partner?.name} actions={actions} />
            ))}
          </ul>
        )}
      </section>

      <Composer open={composing} onOpenChange={setComposing} me={me} partner={partner} actions={actions} />
    </>
  )
}
