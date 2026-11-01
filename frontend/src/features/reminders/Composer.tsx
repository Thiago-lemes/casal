import * as Dialog from '@radix-ui/react-dialog'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, TextField } from '@/design-system'
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
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className={styles.overlay} />
        <Dialog.Content className={styles.sheet} aria-describedby={undefined}>
          <Dialog.Title className={styles.title}>{t('composer.title')}</Dialog.Title>
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
              <fieldset className={styles.group}>
                <legend className={styles.legend}>{t('composer.to')}</legend>
                <div className={styles.options}>
                  {[
                    { id: me.id, label: t('composer.me') },
                    { id: partner.id, label: partner.name },
                  ].map((o) => (
                    <label key={o.id} className={styles.option}>
                      <input
                        type="radio"
                        name="recipient"
                        value={o.id}
                        checked={recipient === o.id}
                        onChange={() => setRecipient(o.id)}
                      />
                      <span>{o.label}</span>
                    </label>
                  ))}
                </div>
              </fieldset>
            )}

            <fieldset className={styles.group}>
              <legend className={styles.legend}>{t('composer.when')}</legend>
              <div className={styles.options}>
                {choices.map((c) => (
                  <label key={c} className={styles.option}>
                    <input type="radio" name="when" value={c} checked={when === c} onChange={() => setWhen(c)} />
                    <span>{t(`composer.${c}`)}</span>
                  </label>
                ))}
              </div>
            </fieldset>

            {actions.create.isError && (
              <p className={styles.error} role="alert">
                {errorMessage(tc, actions.create.error)}
              </p>
            )}

            <div className={styles.footer}>
              <Dialog.Close asChild>
                <Button variant="quiet">{tc('actions.cancel')}</Button>
              </Dialog.Close>
              <Button type="submit" variant="primary" loading={actions.create.isPending}>
                {t('composer.create')}
              </Button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
