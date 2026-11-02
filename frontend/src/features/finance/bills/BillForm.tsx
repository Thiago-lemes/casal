import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, Dialog, DialogClose, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Bill, BillInput } from '../api'
import { CategorySelect } from '../CategorySelect'
import { selectable } from '../categories'
import { todayIn } from '../dates'
import { useBillActions, useCategories } from '../hooks'
import { centsToInput, interpretedHint, MAX_CENTS, parseMoney } from '../money'
import type { Ctx } from '../types'
import styles from '../finance.module.css'

/** Nova conta ou edição de uma conta aberta. Os lembretes são automáticos (3 dias antes e no dia): aqui só se explica. */
export function BillForm({ ctx, bill, onClose, onSaved }: { ctx: Ctx; bill?: Bill; onClose: () => void; onSaved?: (b: Bill) => void }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const actions = useBillActions()
  const categories = useCategories(Boolean(bill))
  const list = selectable(categories.data ?? [], bill?.category_id)

  const [title, setTitle] = useState(bill?.title ?? '')
  const [amount, setAmount] = useState(bill?.amount_cents ? centsToInput(bill.amount_cents) : '')
  const [dueOn, setDueOn] = useState(bill?.due_on ?? todayIn(ctx.me.timezone))
  const [responsible, setResponsible] = useState(bill?.responsible_id ?? ctx.me.id)
  const [categoryId, setCategoryId] = useState(bill?.category_id ?? '')
  const [notifyBoth, setNotifyBoth] = useState(bill?.notify_both ?? false)
  const [repeat, setRepeat] = useState<'none' | 'monthly'>(bill?.repeat ?? 'none')

  // "Contas fixas" é a categoria padrão de uma conta
  const category = categoryId || list.find((c) => c.key === 'bills')?.id || list[0]?.id || ''
  const cents = amount.trim() === '' ? null : parseMoney(amount)
  const amountBad = amount.trim() !== '' && (cents === null || cents <= 0 || cents > MAX_CENTS)
  const valid = title.trim() !== '' && Boolean(dueOn) && Boolean(category) && !amountBad

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid) return
    const body: BillInput = {
      title: title.trim(),
      amount_cents: cents,
      due_on: dueOn,
      responsible_id: responsible,
      category_id: category,
      notify_both: notifyBoth,
      repeat,
    }
    const done = {
      onSuccess: (b: Bill) => {
        onSaved?.(b)
        onClose()
      },
    }
    if (bill) actions.update.mutate({ id: bill.id, body }, done)
    else actions.create.mutate(body, done)
  }

  const error = actions.create.error ?? actions.update.error ?? actions.remove.error

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={bill ? t('bills.editTitle') : t('bills.newTitle')}>
      <form className={styles.form} onSubmit={submit} noValidate>
        <TextField label={t('bills.titleLabel')} value={title} maxLength={120} onChange={(e) => setTitle(e.target.value)} required />
        <TextField
          label={t('bills.amount')}
          inputMode="decimal"
          autoComplete="off"
          placeholder="0,00"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          hint={interpretedHint(t, cents, i18n.language) ?? t('bills.amountHint')}
          error={amountBad ? t('form.amountInvalid') : undefined}
        />
        <TextField label={t('bills.dueOn')} type="date" value={dueOn} onChange={(e) => setDueOn(e.target.value)} required />
        {ctx.partner && (
          <ChoiceGroup
            legend={t('bills.responsible')}
            options={ctx.people.map((p) => ({ value: p.id, label: p.id === ctx.me.id ? t('form.me') : p.name }))}
            value={responsible}
            onChange={setResponsible}
          />
        )}
        <CategorySelect categories={list} value={category} onChange={setCategoryId} />
        {ctx.partner && (
          <label className={styles.check}>
            <input type="checkbox" checked={notifyBoth} onChange={(e) => setNotifyBoth(e.target.checked)} />
            <span>{t('bills.notifyBoth')}</span>
          </label>
        )}
        <ChoiceGroup
          legend={t('bills.repeat')}
          options={[
            { value: 'none', label: t('bills.repeatNone') },
            { value: 'monthly', label: t('bills.repeatMonthly') },
          ]}
          value={repeat}
          onChange={setRepeat}
        />
        <p className={styles.hint}>{t('bills.remindersHint')}</p>
        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}
        <div className={styles.footer}>
          {bill && (
            <Button
              variant="quiet"
              loading={actions.remove.isPending}
              onClick={() => actions.remove.mutate(bill.id, { onSuccess: onClose })}
            >
              {t('detail.delete')}
            </Button>
          )}
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={actions.create.isPending || actions.update.isPending} disabled={!valid}>
            {t('composer.save')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
