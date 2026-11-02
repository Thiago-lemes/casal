import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Occurrence } from './api'
import { formatClock, formatDay } from './dates'
import styles from './OccurrenceDialog.module.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  occurrence: Occurrence
  /** o dia (AAAA-MM-DD) em que a ocorrência foi tocada */
  day: string
  timezone: string
  /** só quem pode editar o bloco abre exceções */
  canEdit: boolean
  pending?: boolean
  error?: unknown
  onSkip: () => void
  onChangeTime: (start: string, end: string) => void
  onClear: () => void
  onEditBlock: () => void
}

/** Uma ocorrência num dia: ver os detalhes, abrir uma exceção (folga ou outro horário) ou ir ao bloco. */
export function OccurrenceDialog({ open, onOpenChange, occurrence: o, day, timezone, canEdit, pending, error, onSkip, onChangeTime, onClear, onEditBlock }: Props) {
  const { t, i18n } = useTranslation('routine')
  const { t: tc } = useTranslation('common')
  const [changing, setChanging] = useState(false)
  const [start, setStart] = useState(() => formatHm(o.start_at, timezone))
  const [end, setEnd] = useState(() => formatHm(o.end_at, timezone))

  const bad = !start || !end || end <= start

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!bad) onChangeTime(start, end)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={o.busy ? t('busy') : o.title}>
      <div className={styles.body}>
        <p className={styles.when}>
          {formatDay(day, i18n.language)}
          <br />
          {formatClock(o.start_at, i18n.language, timezone)} – {formatClock(o.end_at, i18n.language, timezone)}
          {o.exception && ` · ${t('exception.badge')}`}
        </p>
        {!o.busy && o.location && <p className={styles.where}>{o.location}</p>}
        <p className={styles.who}>{o.owner_name}</p>

        {canEdit && !changing && (
          <div className={styles.actions}>
            <Button onClick={onSkip} loading={pending}>
              {t('exception.skip')}
            </Button>
            <Button onClick={() => setChanging(true)}>{t('exception.change')}</Button>
            {o.exception && (
              <Button variant="quiet" onClick={onClear} loading={pending}>
                {t('exception.clear')}
              </Button>
            )}
            <Button variant="quiet" onClick={onEditBlock}>
              {t('exception.editBlock')}
            </Button>
          </div>
        )}

        {canEdit && changing && (
          <form className={styles.form} onSubmit={submit}>
            <p className={styles.hint}>{t('exception.changeHint')}</p>
            <div className={styles.row}>
              <TextField label={t('editor.start')} type="time" value={start} onChange={(e) => setStart(e.target.value)} />
              <TextField label={t('editor.end')} type="time" value={end} onChange={(e) => setEnd(e.target.value)} error={bad ? t('editor.problem.time') : undefined} />
            </div>
            <div className={styles.footer}>
              <Button variant="quiet" onClick={() => setChanging(false)}>
                {tc('actions.cancel')}
              </Button>
              <Button type="submit" variant="primary" loading={pending} disabled={bad}>
                {t('exception.apply')}
              </Button>
            </div>
          </form>
        )}

        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}

        {!changing && (
          <div className={styles.footer}>
            <DialogClose asChild>
              <Button variant="quiet">{tc('actions.close')}</Button>
            </DialogClose>
          </div>
        )}
      </div>
    </Dialog>
  )
}

/** "HH:MM" de um instante no fuso, para os campos de hora. */
function formatHm(iso: string, timeZone: string): string {
  return new Intl.DateTimeFormat('en-GB', { timeZone, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(new Date(iso))
}
