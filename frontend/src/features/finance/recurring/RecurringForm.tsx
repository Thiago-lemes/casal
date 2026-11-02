import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, Dialog, DialogClose, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Recurring, RecurringInput, Split } from '../api'
import { CategorySelect } from '../CategorySelect'
import { selectable } from '../categories'
import { monthOf, todayIn } from '@/shared/lib/dates'
import { SplitChoice } from '../expenses/SplitChoice'
import { useCategories, useRecurringActions } from '../hooks'
import { centsToInput, interpretedHint, MAX_CENTS, parseMoney } from '@/shared/lib/money'
import { equalSplit, splitFor } from '../split'
import type { SplitMode } from '../split'
import type { Ctx } from '../types'
import styles from '../finance.module.css'

/** Criar ou editar um gasto fixo. Editar vale dos próximos meses em diante; o que já foi gerado não muda. */
export function RecurringForm({ ctx, recurring, onClose }: { ctx: Ctx; recurring?: Recurring; onClose: () => void }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const actions = useRecurringActions()
  const categories = useCategories(Boolean(recurring))
  const today = todayIn(ctx.me.timezone)
  const monthStart = `${monthOf(today)}-01`

  const [scope, setScope] = useState<'household' | 'personal'>(recurring?.scope ?? 'household')
  const [amount, setAmount] = useState(recurring ? centsToInput(recurring.amount_cents) : '')
  const [categoryId, setCategoryId] = useState(recurring?.category_id ?? '')
  const [description, setDescription] = useState(recurring?.description ?? '')
  const [paidBy, setPaidBy] = useState(recurring?.paid_by ?? ctx.me.id)
  const [mode, setMode] = useState<SplitMode>(recurring?.split ? 'custom' : 'default')
  const [custom, setCustom] = useState<Split>(recurring?.split ?? equalSplit(ctx.people))
  const [visibility, setVisibility] = useState<'private' | 'view'>(recurring?.visibility === 'view' ? 'view' : 'private')
  const [day, setDay] = useState(String(recurring?.day_of_month ?? Number(today.slice(8))))
  const [startOn, setStartOn] = useState(recurring?.start_on ?? today)
  const [endOn, setEndOn] = useState(recurring?.end_on ?? '')

  const list = selectable(categories.data ?? [], recurring?.category_id)
  const category = categoryId || list[0]?.id || ''
  const cents = parseMoney(amount)
  const dayNum = Number(day)
  const dayOk = Number.isInteger(dayNum) && dayNum >= 1 && dayNum <= 31
  const amountBad = amount.trim() !== '' && (cents === null || cents <= 0 || cents > MAX_CENTS)
  const startBad = !recurring && startOn < monthStart
  const valid = cents !== null && cents > 0 && !amountBad && dayOk && Boolean(category) && Boolean(startOn) && !startBad
  const household = scope === 'household'

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid) return
    const body: RecurringInput = {
      scope,
      amount_cents: cents!,
      category_id: category,
      description: description.trim(),
      day_of_month: dayNum,
      start_on: startOn,
      ...(endOn ? { end_on: endOn } : {}),
    }
    if (household) {
      body.paid_by = paidBy
      const split = ctx.partner ? splitFor(mode, ctx.people, ctx.me.id, custom) : undefined
      if (split) body.split = split
    } else {
      body.visibility = visibility
    }
    const done = { onSuccess: onClose }
    if (recurring) actions.update.mutate({ id: recurring.id, body }, done)
    else actions.create.mutate(body, done)
  }

  const error = actions.create.error ?? actions.update.error

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={recurring ? t('recurring.editTitle') : t('recurring.newTitle')}>
      <form className={styles.form} onSubmit={submit} noValidate>
        {!recurring && (
          <ChoiceGroup
            legend={t('composer.scope')}
            options={[
              { value: 'household', label: t('scope.household') },
              { value: 'personal', label: t('scope.personal') },
            ]}
            value={scope}
            onChange={setScope}
          />
        )}
        <TextField
          label={t('form.amount')}
          inputMode="decimal"
          autoComplete="off"
          placeholder="0,00"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          error={amountBad ? t('form.amountInvalid') : undefined}
          hint={interpretedHint(t, cents, i18n.language)}
          required
        />
        <CategorySelect categories={list} value={category} onChange={setCategoryId} />
        <TextField label={t('form.description')} value={description} maxLength={140} onChange={(e) => setDescription(e.target.value)} />
        <TextField
          label={t('recurring.day')}
          type="number"
          inputMode="numeric"
          min={1}
          max={31}
          value={day}
          onChange={(e) => setDay(e.target.value)}
          hint={t('recurring.dayHint')}
          error={day !== '' && !dayOk ? t('recurring.dayInvalid') : undefined}
        />
        <div className={styles.row2}>
          <TextField
            label={t('recurring.startOn')}
            type="date"
            min={recurring ? undefined : monthStart}
            value={startOn}
            onChange={(e) => setStartOn(e.target.value)}
            error={startBad ? t('recurring.startInvalid') : undefined}
          />
          <TextField label={t('recurring.endOn')} type="date" min={startOn} value={endOn} onChange={(e) => setEndOn(e.target.value)} />
        </div>

        {household ? (
          <SplitChoice ctx={ctx} paidBy={paidBy} onPaidBy={setPaidBy} mode={mode} onMode={setMode} custom={custom} onCustom={setCustom} />
        ) : (
          <ChoiceGroup
            legend={t('visibility.legend')}
            options={[
              { value: 'private', label: t('visibility.private') },
              { value: 'view', label: t('visibility.view') },
            ]}
            value={visibility}
            onChange={setVisibility}
          />
        )}

        <p className={styles.hint}>{t('recurring.editHint')}</p>
        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}
        <div className={styles.footer}>
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
