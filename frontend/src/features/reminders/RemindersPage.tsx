import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, EmptyState, Skeleton, Tabs } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { formatTime } from '@/shared/lib/time'
import { isPending } from './api'
import type { Reminder } from './api'
import { Composer } from './Composer'
import { useReminderActions, useReminders } from './hooks'
import { ReminderLabel } from './ReminderLabel'
import styles from './RemindersPage.module.css'

type Tab = 'pending' | 'resolved'
const TABS: Tab[] = ['pending', 'resolved']

type Props = {
  me: { id: string; timezone: string }
  partner?: { id: string; name: string }
}

/** Todos os lembretes entre o casal: o que ainda pede uma ação e o que já foi resolvido. */
export function RemindersPage({ me, partner }: Props) {
  const { t, i18n } = useTranslation('reminders')
  const { t: tc } = useTranslation('common')
  const reminders = useReminders(true)
  const actions = useReminderActions()
  const [tab, setTab] = useState<Tab>('pending')
  const [composing, setComposing] = useState(false)

  const all = reminders.data
  const pending = useMemo(
    () => (all ?? []).filter(isPending).sort((a, b) => a.remind_at.localeCompare(b.remind_at)),
    [all],
  )
  const resolved = useMemo(
    () =>
      (all ?? [])
        .filter((r) => r.status === 'resolved')
        .sort((a, b) => (b.resolved_at ?? '').localeCompare(a.resolved_at ?? '')),
    [all],
  )
  const shown = tab === 'pending' ? pending : resolved

  const day = (iso: string) =>
    new Intl.DateTimeFormat(i18n.language, { weekday: 'short', day: 'numeric', month: 'short', timeZone: me.timezone }).format(new Date(iso))

  return (
    <div className={styles.page}>
      <header className={styles.hero}>
        <div>
          <h1 className={styles.title}>{t('title')}</h1>
          {reminders.isSuccess && <p className={styles.count}>{t('pendingCount', { count: pending.length })}</p>}
        </div>
        <Button variant="primary" className={styles.cta} onClick={() => setComposing(true)}>
          {t('new')}
        </Button>
      </header>

      <Tabs label={t('tabs.label')} value={tab} onChange={setTab} tabs={TABS.map((v) => ({ value: v, label: t(`tabs.${v}`) }))}>
        <div aria-live="polite">
          {reminders.isPending && (
            <div className={styles.skeletons}>
              <Skeleton />
              <Skeleton />
            </div>
          )}

          {reminders.isError && (
            <EmptyState
              title={t('loadError')}
              body={errorMessage(tc, reminders.error)}
              action={<Button onClick={() => void reminders.refetch()}>{tc('actions.tryAgain')}</Button>}
            />
          )}

          {reminders.isSuccess && shown.length === 0 && (
            <EmptyState
              title={t(`empty.${tab}.title`)}
              body={t(`empty.${tab}.body`)}
              action={
                tab === 'pending' ? (
                  <Button variant="primary" onClick={() => setComposing(true)}>
                    {t('new')}
                  </Button>
                ) : undefined
              }
            />
          )}

          {tab === 'pending' && pending.length > 0 && (
            <ul>
              {pending.map((r) => (
                <ReminderLabel key={r.id} reminder={r} me={me} partnerName={partner?.name} actions={actions} />
              ))}
            </ul>
          )}

          {tab === 'resolved' && resolved.length > 0 && (
            <ul className={styles.resolved}>
              {resolved.map((r: Reminder) => (
                <li key={r.id} className={styles.row}>
                  <p className={styles.body}>{r.body}</p>
                  <p className={styles.meta}>
                    {r.created_by === me.id ? t('byYou') : t('by', { name: r.created_by_name })}
                    {r.recipient_id !== me.id && r.created_by === me.id && partner ? ` · ${t('forPartner', { name: partner.name })}` : ''}
                  </p>
                  {r.resolved_at && (
                    <p className={styles.when}>
                      {t('resolvedAt', { when: `${day(r.resolved_at)}, ${formatTime(r.resolved_at, i18n.language, me.timezone)}` })}
                    </p>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      </Tabs>

      <Composer open={composing} onOpenChange={setComposing} me={me} partner={partner} actions={actions} />
    </div>
  )
}
