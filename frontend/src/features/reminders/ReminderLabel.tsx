import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { formatTime, isPast, minutesUntilTomorrowMorning } from '@/shared/lib/time'
import type { Reminder } from './api'
import type { useReminderActions } from './hooks'
import styles from './ReminderLabel.module.css'

type Actions = ReturnType<typeof useReminderActions>
type Phase = 'idle' | 'stamped' | 'leaving'

type Props = {
  reminder: Reminder
  me: { id: string; timezone: string }
  partnerName?: string
  actions: Actions
}

const STAMP_MS = 900

/**
 * Um lembrete como etiqueta de freezer: hora grande à esquerda, texto, quem deixou.
 * "Dar baixa" carimba a hora da resolução, fecha a etiqueta e só então atualiza a lista.
 */
export function ReminderLabel({ reminder, me, partnerName, actions }: Props) {
  const { t, i18n } = useTranslation('today')
  const { t: tc } = useTranslation('common')
  const [phase, setPhase] = useState<Phase>('idle')
  const [stampTime, setStampTime] = useState('')
  const [snoozing, setSnoozing] = useState(false)
  const [error, setError] = useState<string>()

  const mine = reminder.recipient_id === me.id
  const creator = reminder.created_by === me.id ? t('fromYou') : t('from', { name: reminder.created_by_name })
  const late = isPast(reminder.remind_at)
  const time = formatTime(reminder.remind_at, i18n.language, me.timezone)

  function fail(err: unknown) {
    setPhase('idle')
    setError(errorMessage(tc, err))
  }

  function resolve() {
    setError(undefined)
    actions.resolve.mutate(reminder.id, {
      onSuccess: () => {
        setStampTime(formatTime(new Date().toISOString(), i18n.language, me.timezone))
        setPhase('stamped')
        window.setTimeout(() => setPhase('leaving'), STAMP_MS)
      },
      onError: fail,
    })
  }

  function snooze(minutes: number) {
    setError(undefined)
    setSnoozing(false)
    actions.snooze.mutate({ id: reminder.id, minutes }, { onSuccess: () => void actions.refresh(), onError: fail })
  }

  function cancel() {
    setError(undefined)
    actions.cancel.mutate(reminder.id, { onSuccess: () => setPhase('leaving'), onError: fail })
  }

  return (
    <li
      className={styles.item}
      data-phase={phase}
      onAnimationEnd={(e) => {
        if (phase === 'leaving' && e.target === e.currentTarget) void actions.refresh()
      }}
    >
      <div className={styles.label}>
        <p className={styles.time} data-late={late || undefined}>
          {time}
        </p>
        <div className={styles.body}>
          <p className={styles.text}>{reminder.body}</p>
          <p className={styles.meta}>
            {creator}
            {!mine && partnerName ? ` · ${t('forPartner', { name: partnerName })}` : ''}
            {late && <span className={styles.late}> · {t('late')}</span>}
          </p>
          {error && (
            <p className={styles.error} role="alert">
              {error}
            </p>
          )}
          {phase === 'idle' && (
            <div className={styles.actions}>
              {mine && (
                <>
                  <Button variant="primary" loading={actions.resolve.isPending} onClick={resolve}>
                    {t('resolve')}
                  </Button>
                  <Button variant="secondary" onClick={() => setSnoozing((v) => !v)} aria-expanded={snoozing}>
                    {t('snooze')}
                  </Button>
                </>
              )}
              {reminder.created_by === me.id && (
                <Button variant="quiet" onClick={cancel}>
                  {t('cancelReminder')}
                </Button>
              )}
            </div>
          )}
          {snoozing && phase === 'idle' && (
            <div className={styles.actions}>
              <Button variant="quiet" onClick={() => snooze(10)}>
                {t('snooze10')}
              </Button>
              <Button variant="quiet" onClick={() => snooze(60)}>
                {t('snooze60')}
              </Button>
              <Button variant="quiet" onClick={() => snooze(minutesUntilTomorrowMorning())}>
                {t('snoozeTomorrow')}
              </Button>
            </div>
          )}
        </div>
        {phase !== 'idle' && stampTime && (
          <p className={styles.stamp} role="status">
            {t('resolved', { time: stampTime })}
          </p>
        )}
      </div>
    </li>
  )
}
