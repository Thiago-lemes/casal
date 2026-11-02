import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { formatDay, todayIn } from '@/shared/lib/dates'
import type { Goal } from '../api'
import { pastDays } from '../days'
import { useCheckinActions } from '../hooks'
import type { Me } from '../types'
import styles from '../goals.module.css'

/**
 * Marcar hoje (idempotente: marcar duas vezes o mesmo dia conta uma) ou outro dia dos últimos 31.
 * `progress.marked_today` vem do servidor e decide o rótulo; o botão anuncia o estado com `aria-pressed`.
 */
export function MarkControls({ goal, me }: { goal: Goal; me: Me }) {
  const { t, i18n } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const { mark, undo } = useCheckinActions(goal.id)
  const [other, setOther] = useState(false)
  const [day, setDay] = useState('')
  const dayId = useId()
  const today = todayIn(me.timezone)
  const marked = goal.progress.marked_today === true
  const error = mark.error ?? undo.error

  return (
    <div className={styles.form}>
      <Button
        variant={marked ? 'secondary' : 'primary'}
        className={styles.mark}
        aria-pressed={marked}
        loading={mark.isPending}
        disabled={marked}
        onClick={() => mark.mutate(undefined)}
      >
        {t(marked ? 'mark.marked' : 'mark.today')}
      </Button>

      <div className={styles.actions}>
        {marked && (
          <Button variant="quiet" loading={undo.isPending} onClick={() => undo.mutate(today)}>
            {t('mark.undo')}
          </Button>
        )}
        <Button variant="quiet" aria-expanded={other} onClick={() => setOther(!other)}>
          {t('mark.otherDay')}
        </Button>
      </div>

      {other && (
        <div className={styles.inline}>
          <div className={styles.field}>
            <label htmlFor={dayId} className={styles.label}>
              {t('mark.pickDay')}
            </label>
            <select id={dayId} className={styles.select} value={day} onChange={(e) => setDay(e.target.value)}>
              <option value="">{t('mark.pickPlaceholder')}</option>
              {pastDays(today).map((d) => (
                <option key={d} value={d}>
                  {formatDay(d, i18n.language, { weekday: 'short', day: 'numeric', month: 'short' })}
                </option>
              ))}
            </select>
          </div>
          <Button
            disabled={!day}
            loading={mark.isPending}
            onClick={() => mark.mutate(day, { onSuccess: () => setDay('') })}
          >
            {t('mark.markDay')}
          </Button>
        </div>
      )}

      {error != null && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, error)}
        </p>
      )}
    </div>
  )
}
