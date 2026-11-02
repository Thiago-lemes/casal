import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { AssigneeSelect } from './AssigneeSelect'
import type { Choice } from './permissions'
import type { Person } from './types'
import styles from './TaskDialogs.module.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  partner?: Person
  onSubmit: (choice: Choice) => void
  pending?: boolean
  error?: unknown
}

export function ReassignDialog({ open, onOpenChange, partner, onSubmit, pending, error }: Props) {
  const { t } = useTranslation('tasks')
  const { t: tc } = useTranslation('common')
  const [choice, setChoice] = useState<Choice>(partner ? 'partner' : 'shared')

  function submit(e: FormEvent) {
    e.preventDefault()
    onSubmit(choice)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t('reassign.title')}>
      <form className={styles.form} onSubmit={submit}>
        <AssigneeSelect value={choice} onChange={setChoice} partner={partner} />

        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}

        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={pending}>
            {t('reassign.submit')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
