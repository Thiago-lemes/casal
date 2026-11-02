import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, Dialog, DialogClose, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { todayIn } from '../dates'
import { useExpenseActions } from '../hooks'
import { interpretedHint, MAX_CENTS, parseMoney } from '../money'
import type { Ctx } from '../types'
import styles from '../finance.module.css'

/**
 * Formulário simplificado depois da compra no mercado: valor, quem pagou e descrição (a categoria é Mercado).
 * O backend de Finanças não lê a lista de compras; quem leva o valor é a pessoa que o digita.
 */
export function FromShoppingDialog({ ctx, onClose, onSaved }: { ctx: Ctx; onClose: () => void; onSaved?: (fallback: boolean) => void }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const actions = useExpenseActions()
  const [amount, setAmount] = useState('')
  const [paidBy, setPaidBy] = useState(ctx.me.id)
  const [description, setDescription] = useState('')

  const cents = parseMoney(amount)
  const bad = amount.trim() !== '' && (cents === null || cents <= 0 || cents > MAX_CENTS)
  const valid = cents !== null && cents > 0 && !bad

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid) return
    actions.fromShopping.mutate(
      { amount_cents: cents!, paid_by: paidBy, description: description.trim(), spent_on: todayIn(ctx.me.timezone) },
      {
        onSuccess: (res) => {
          onSaved?.(Boolean(res.split_fallback))
          onClose()
        },
      },
    )
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('shopping.title')}>
      <form className={styles.form} onSubmit={submit} noValidate>
        <TextField
          label={t('form.amount')}
          inputMode="decimal"
          autoComplete="off"
          placeholder="0,00"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          error={bad ? t('form.amountInvalid') : undefined}
          hint={interpretedHint(t, cents, i18n.language)}
          required
        />
        {ctx.partner && (
          <ChoiceGroup
            legend={t('form.paidBy')}
            options={ctx.people.map((p) => ({ value: p.id, label: p.id === ctx.me.id ? t('form.me') : p.name }))}
            value={paidBy}
            onChange={setPaidBy}
          />
        )}
        <TextField label={t('form.description')} value={description} maxLength={140} onChange={(e) => setDescription(e.target.value)} />
        <p className={styles.hint}>{t('shopping.hint')}</p>
        {actions.fromShopping.error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, actions.fromShopping.error)}
          </p>
        )}
        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={actions.fromShopping.isPending} disabled={!valid}>
            {t('composer.submit')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
