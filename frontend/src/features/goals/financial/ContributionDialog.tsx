import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { todayIn } from '@/shared/lib/dates'
import { formatBRL, MAX_CENTS, parseMoney } from '@/shared/lib/money'
import type { Goal } from '../api'
import { useContributionActions } from '../hooks'
import type { Me } from '../types'
import styles from '../goals.module.css'

/** Aportar e retirar usam o mesmo diálogo: o valor é sempre positivo e o botão decide o sinal. */
export function ContributionDialog({ goal, me, mode, onClose }: { goal: Goal; me: Me; mode: 'add' | 'withdraw'; onClose: () => void }) {
  const { t, i18n } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const { add } = useContributionActions(goal.id)
  const today = todayIn(me.timezone)
  const [amount, setAmount] = useState('')
  const [madeOn, setMadeOn] = useState(today)
  const [note, setNote] = useState('')
  const [invalid, setInvalid] = useState(false)

  const cents = parseMoney(amount)
  const valid = cents !== null && cents > 0 && cents <= MAX_CENTS
  const hint = valid ? t('form.interpreted', { amount: formatBRL(cents, i18n.language) }) : undefined

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid) return setInvalid(true)
    setInvalid(false)
    add.mutate(
      { amount_cents: mode === 'withdraw' ? -cents : cents, made_on: madeOn || undefined, note: note.trim() || undefined },
      { onSuccess: onClose },
    )
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()} title={t(mode === 'add' ? 'contribution.addTitle' : 'contribution.withdrawTitle')}>
      <form className={styles.form} onSubmit={submit} noValidate>
        <p className={styles.notice}>{t('contribution.notice')}</p>
        <TextField
          label={t('contribution.amount')}
          inputMode="decimal"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          hint={hint}
          error={invalid ? t('form.errors.amount') : undefined}
          autoFocus
          required
        />
        <TextField label={t('contribution.date')} type="date" max={today} value={madeOn} onChange={(e) => setMadeOn(e.target.value)} />
        <TextField label={t('contribution.note')} maxLength={200} value={note} onChange={(e) => setNote(e.target.value)} />
        {add.error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, add.error)}
          </p>
        )}
        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={add.isPending}>
            {t(mode === 'add' ? 'contribution.add' : 'contribution.withdraw')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
