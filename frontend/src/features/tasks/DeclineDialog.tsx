import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose, TextArea } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import styles from './TaskDialogs.module.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** recebe o motivo já sem espaços nas pontas */
  onSubmit: (reason: string) => void
  pending?: boolean
  error?: unknown
}

export const MAX_REASON = 500

/** Recusar exige justificativa: o botão só habilita com texto. */
export function DeclineDialog({ open, onOpenChange, onSubmit, pending, error }: Props) {
  const { t } = useTranslation('tasks')
  const { t: tc } = useTranslation('common')
  const [reason, setReason] = useState('')
  const trimmed = reason.trim()

  function submit(e: FormEvent) {
    e.preventDefault()
    if (trimmed) onSubmit(trimmed)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t('decline.title')}>
      <form className={styles.form} onSubmit={submit}>
        <p className={styles.lead}>{t('decline.lead')}</p>
        <TextArea label={t('decline.reason')} value={reason} maxLength={MAX_REASON} onChange={(e) => setReason(e.target.value)} autoFocus />

        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}

        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={pending} disabled={!trimmed}>
            {t('decline.submit')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
