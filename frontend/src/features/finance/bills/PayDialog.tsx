import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, Dialog, DialogClose, TextField } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Bill, PayResult } from '../api'
import { todayIn } from '@/shared/lib/dates'
import { useBillActions } from '../hooks'
import { centsToInput, formatBRL, interpretedHint, MAX_CENTS, parseMoney } from '@/shared/lib/money'
import type { Ctx } from '../types'
import styles from '../finance.module.css'

/** Pagar uma conta cria o gasto do casal com a divisão padrão e liga à conta. Sem valor na conta, o valor é obrigatório. */
export function PayDialog({ ctx, bill, onClose, onPaid }: { ctx: Ctx; bill: Bill; onClose: () => void; onPaid?: (r: PayResult) => void }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const actions = useBillActions()
  const [amount, setAmount] = useState(bill.amount_cents ? centsToInput(bill.amount_cents) : '')
  const [paidOn, setPaidOn] = useState(todayIn(ctx.me.timezone))
  const [paidBy, setPaidBy] = useState(ctx.me.id)

  const cents = parseMoney(amount)
  const needsAmount = !bill.amount_cents
  const amountBad = amount.trim() !== '' && (cents === null || cents <= 0 || cents > MAX_CENTS)
  const valid = !amountBad && (!needsAmount || (cents !== null && cents > 0)) && Boolean(paidOn)

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid) return
    actions.pay.mutate(
      { id: bill.id, body: { ...(cents ? { amount_cents: cents } : {}), paid_on: paidOn, paid_by: paidBy } },
      {
        onSuccess: (res) => {
          onPaid?.(res)
          onClose()
        },
        onError: (err) => {
          // alguém pagou antes: recarrega
          if (err instanceof ApiError && err.is('bill_already_paid')) void actions.reload()
        },
      },
    )
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('pay.title', { title: bill.title })}>
      <form className={styles.form} onSubmit={submit} noValidate>
        {bill.amount_cents ? <p style={{ fontSize: 'var(--text-xl)', fontWeight: 700 }}>{formatBRL(bill.amount_cents, i18n.language)}</p> : null}
        <TextField
          label={needsAmount ? t('pay.amountRequired') : t('pay.amount')}
          inputMode="decimal"
          autoComplete="off"
          placeholder="0,00"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          error={amountBad ? t('form.amountInvalid') : undefined}
          hint={interpretedHint(t, cents, i18n.language)}
          required={needsAmount}
        />
        <TextField label={t('pay.paidOn')} type="date" value={paidOn} onChange={(e) => setPaidOn(e.target.value)} required />
        {ctx.partner && (
          <ChoiceGroup
            legend={t('form.paidBy')}
            options={ctx.people.map((p) => ({ value: p.id, label: p.id === ctx.me.id ? t('form.me') : p.name }))}
            value={paidBy}
            onChange={setPaidBy}
          />
        )}
        <p className={styles.hint}>{t('pay.hint')}</p>
        {actions.pay.error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, actions.pay.error)}
          </p>
        )}
        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={actions.pay.isPending} disabled={!valid}>
            {t('pay.confirm')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
