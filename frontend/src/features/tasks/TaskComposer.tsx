import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { AssigneeSelect } from './AssigneeSelect'
import type { TaskActionsApi } from './hooks'
import { toAssignment } from './permissions'
import type { Choice } from './permissions'
import type { Me, Person } from './types'
import styles from './TaskDialogs.module.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  me: Me
  partner?: Person
  actions: TaskActionsApi
}

export function TaskComposer({ open, onOpenChange, me, partner, actions }: Props) {
  const { t } = useTranslation('tasks')
  const { t: tc } = useTranslation('common')
  const [title, setTitle] = useState('')
  const [choice, setChoice] = useState<Choice>('me')
  const [due, setDue] = useState('')

  function submit(e: FormEvent) {
    e.preventDefault()
    actions.create.mutate(
      {
        title: title.trim(),
        ...toAssignment(choice, me.id, partner?.id),
        // datetime-local vem sem fuso: vale no horário do aparelho
        ...(due ? { due_at: new Date(due).toISOString() } : {}),
      },
      {
        onSuccess: () => {
          setTitle('')
          setChoice('me')
          setDue('')
          onOpenChange(false)
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t('composer.title')}>
      <form className={styles.form} onSubmit={submit}>
        <TextField
          label={t('composer.name')}
          placeholder={t('composer.placeholder')}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          required
          maxLength={200}
          autoFocus
        />
        <AssigneeSelect value={choice} onChange={setChoice} partner={partner} />
        <TextField label={t('due.label')} type="datetime-local" value={due} onChange={(e) => setDue(e.target.value)} />

        {actions.create.isError && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, actions.create.error)}
          </p>
        )}

        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={actions.create.isPending} disabled={!title.trim()}>
            {t('composer.create')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
