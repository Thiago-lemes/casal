import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, Dialog, DialogClose, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { quickTimes } from '@/shared/lib/time'
import type { useReminderActions } from './hooks'
import styles from './Composer.module.css'

type Choice = 'inOneHour' | 'tonight' | 'tomorrowMorning'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  me: { id: string }
  partner?: { id: string; name: string }
  actions: ReturnType<typeof useReminderActions>
}

export function Composer({ open, onOpenChange, me, partner, actions }: Props) {
  const { t } = useTranslation('today')
  const { t: tc } = useTranslation('common')
  const [body, setBody] = useState('')
  const [recipient, setRecipient] = useState(me.id)
  const [when, setWhen] = useState<Choice>('inOneHour')

  function submit(e: FormEvent) {
    e.preventDefault()
    const at = quickTimes()[when]
    actions.create.mutate(
      { recipient_id: recipient, body: body.trim(), remind_at: at.toISOString() },
      {
        onSuccess: () => {
          setBody('')
          setWhen('inOneHour')
          setRecipient(me.id)
          onOpenChange(false)
        },
      },
    )
  }

  const choices: Choice[] = ['inOneHour', 'tonight', 'tomorrowMorning']

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t('composer.title')}>
      <form className={styles.form} onSubmit={submit}>
        <TextField
          label={t('composer.body')}
          placeholder={t('composer.bodyPlaceholder')}
          value={body}
          onChange={(e) => setBody(e.target.value)}
          required
          maxLength={500}
          autoFocus
        />

        {partner && (
          <ChoiceGroup
            legend={t('composer.to')}
            value={recipient}
            onChange={setRecipient}
            options={[
              { value: me.id, label: t('composer.me') },
              { value: partner.id, label: partner.name },
            ]}
          />
        )}

        <ChoiceGroup
          legend={t('composer.when')}
          value={when}
          onChange={setWhen}
          options={choices.map((c) => ({ value: c, label: t(`composer.${c}`) }))}
        />

        {actions.create.isError && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, actions.create.error)}
          </p>
        )}

        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={actions.create.isPending}>
            {t('composer.create')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
