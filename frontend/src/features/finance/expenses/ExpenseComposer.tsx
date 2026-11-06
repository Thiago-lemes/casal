import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, Dialog, DialogClose, TextField } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import { financeApi, isInstallmentsResult } from '../api'
import type { Expense, ExpenseInput, Split } from '../api'
import { categoryName, selectable } from '../categories'
import { todayIn } from '@/shared/lib/dates'
import { useCards, useCategories, useExpenseActions } from '../hooks'
import { centsToInput, formatBRL, interpretedHint, MAX_CENTS, parseMoney } from '@/shared/lib/money'
import { equalSplit, splitFor } from '../split'
import type { SplitMode } from '../split'
import type { Ctx } from '../types'
import { CardField } from './CardField'
import { InstallmentFields, installmentCount } from './InstallmentFields'
import type { InstallmentValue } from './InstallmentFields'
import { SplitEditor } from './SplitEditor'
import styles from '../finance.module.css'

export type Saved = { fallback: boolean; installments: number }

type Props = {
  ctx: Ctx
  open: boolean
  onOpenChange: (open: boolean) => void
  /** com um gasto, é edição (o escopo e a origem não mudam depois de criados) */
  expense?: Expense
  /** categoria sugerida pela chave (ex.: `groceries`, ao vir do modo mercado) */
  presetCategoryKey?: string
  onSaved?: (info: Saved) => void
}

const MAX_DESCRIPTION = 140

/** Registrar ou editar um gasto. O cliente só pré-visualiza: o valor que vale é o da resposta do servidor. */
export function ExpenseComposer({ ctx, open, onOpenChange, expense, presetCategoryKey, onSaved }: Props) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const actions = useExpenseActions()
  const editing = Boolean(expense)
  const categories = useCategories(editing)
  const cards = useCards(editing)
  const today = todayIn(ctx.me.timezone)

  const [scope, setScope] = useState<'household' | 'personal'>(expense?.scope ?? 'household')
  const [amount, setAmount] = useState(expense ? centsToInput(expense.amount_cents) : '')
  const [categoryId, setCategoryId] = useState(expense?.category_id ?? '')
  const [spentOn, setSpentOn] = useState(expense?.spent_on ?? today)
  const [description, setDescription] = useState(expense?.description ?? '')
  const [paidBy, setPaidBy] = useState(expense?.paid_by ?? ctx.me.id)
  const [splitMode, setSplitMode] = useState<SplitMode>(expense?.split ? 'custom' : 'default')
  const [custom, setCustom] = useState<Split>(expense?.split ?? equalSplit(ctx.people))
  const [visibility, setVisibility] = useState<'private' | 'view'>(expense?.visibility === 'view' ? 'view' : 'private')
  const [inst, setInst] = useState<InstallmentValue>({ on: false, count: '2', firstOn: today, mode: 'total' })
  const [cardId, setCardId] = useState(expense?.card_id ?? '')
  const [version, setVersion] = useState(expense?.version ?? 0)
  const [conflict, setConflict] = useState<Expense>()
  const [error, setError] = useState<unknown>()

  const household = scope === 'household'
  const hasPartner = Boolean(ctx.partner)
  // o cartão é de quem pagou: só aparece quando eu pago (e, editando o gasto do parceiro, nunca)
  const showCard = household ? paidBy === ctx.me.id : true
  const list = selectable(categories.data ?? [], expense?.category_id)
  const preset = presetCategoryKey ? list.find((c) => c.key === presetCategoryKey)?.id : undefined
  const category = categoryId || preset || list[0]?.id || ''

  const cents = parseMoney(amount)
  const amountError = amount.trim() !== '' && (cents === null || cents <= 0 || cents > MAX_CENTS) ? t('form.amountInvalid') : undefined
  const perInstallment = inst.on && inst.mode === 'each'
  const count = installmentCount(inst)
  const valid =
    cents !== null && cents > 0 && cents <= MAX_CENTS && Boolean(category) && Boolean(spentOn) && (!inst.on || (count !== null && Boolean(inst.firstOn)))

  function splitToSend(): Split | undefined {
    if (!household || !hasPartner) return expense?.split
    return splitFor(splitMode, ctx.people, ctx.me.id, custom)
  }

  function build(): ExpenseInput {
    const body: ExpenseInput = {
      scope,
      category_id: category,
      spent_on: spentOn,
      description: description.trim(),
    }
    // fora da edição, só envia se escolheu; editando, vazio tira o cartão (quem não vê o campo não o envia)
    if (showCard && (cardId || editing)) body.card_id = cardId
    if (household) {
      body.paid_by = paidBy
      const split = splitToSend()
      if (split) body.split = split
    } else {
      body.visibility = visibility
    }
    if (inst.on && !editing) {
      // parcelar com o valor da parcela não envia `amount_cents`: o servidor usa contagem × valor
      body.installments = { count: count!, first_on: inst.firstOn, ...(perInstallment ? { installment_amount_cents: cents! } : {}) }
      if (!perInstallment) body.amount_cents = cents!
    } else {
      body.amount_cents = cents!
    }
    if (editing) body.version = version
    return body
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid) return
    setError(undefined)
    setConflict(undefined)

    if (expense) {
      actions.update.mutate(
        { id: expense.id, body: build() },
        {
          onSuccess: () => {
            onSaved?.({ fallback: false, installments: 0 })
            onOpenChange(false)
          },
          onError: (err) => void onError(err),
        },
      )
      return
    }
    actions.create.mutate(build(), {
      onSuccess: (res) => {
        onSaved?.({ fallback: Boolean(res.split_fallback), installments: isInstallmentsResult(res) ? res.expenses.length : 0 })
        onOpenChange(false)
      },
      onError: (err) => void onError(err),
    })
  }

  /** Conflito: alguém gravou antes. Mantém o que a pessoa digitou, pega a versão nova e deixa salvar de novo. */
  async function onError(err: unknown) {
    setError(err)
    if (!(err instanceof ApiError)) return
    if (err.is('expense_conflict') && expense) {
      try {
        const fresh = await financeApi.expense(expense.id)
        setVersion(fresh.version)
        setConflict(fresh)
      } catch {
        /* a mensagem de conflito já está na tela */
      }
    }
    if (err.is('expense_settled') || err.is('expense_not_found') || err.is('category_archived')) {
      void actions.reload()
      if (err.is('category_archived')) void categories.refetch()
    }
  }

  const pending = actions.create.isPending || actions.update.isPending
  const splitOptions: { value: SplitMode; label: string }[] = [
    { value: 'default', label: t('split.default') },
    { value: 'equal', label: t('split.equal') },
    { value: 'mine', label: t('split.mine') },
    { value: 'theirs', label: t('split.theirs', { name: ctx.partner?.name ?? '' }) },
    { value: 'custom', label: t('split.custom') },
  ]

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={editing ? t('composer.editTitle') : t('composer.title')}>
      <form className={styles.form} onSubmit={submit} noValidate>
        {!editing && (
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
          label={perInstallment ? t('form.amountEach') : inst.on ? t('form.amountTotal') : t('form.amount')}
          inputMode="decimal"
          autoComplete="off"
          placeholder="0,00"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          error={amountError}
          hint={interpretedHint(t, cents, i18n.language)}
          required
        />

        <div className={styles.row2}>
          <div className={styles.field}>
            <label htmlFor="expense-category" className={styles.label}>
              {t('form.category')}
            </label>
            <select id="expense-category" className={styles.select} value={category} onChange={(e) => setCategoryId(e.target.value)}>
              {list.map((c) => (
                <option key={c.id} value={c.id}>
                  {categoryName(t, c)}
                  {c.archived ? ` (${t('categories.archivedTag')})` : ''}
                </option>
              ))}
            </select>
          </div>
          <TextField label={t('form.date')} type="date" value={spentOn} onChange={(e) => setSpentOn(e.target.value)} required />
        </div>

        <div className={styles.field}>
          <TextField
            label={t('form.description')}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            maxLength={MAX_DESCRIPTION}
          />
          <span className={styles.counter} aria-hidden="true">
            {description.length}/{MAX_DESCRIPTION}
          </span>
        </div>

        {household && hasPartner && (
          <ChoiceGroup
            legend={t('form.paidBy')}
            options={ctx.people.map((p) => ({ value: p.id, label: p.id === ctx.me.id ? t('form.me') : p.name }))}
            value={paidBy}
            onChange={setPaidBy}
          />
        )}

        {showCard && <CardField cards={cards.data ?? []} value={cardId} onChange={setCardId} keep={expense?.card_id} />}

        {household && hasPartner && (
          <>
            <ChoiceGroup legend={t('split.legend')} options={splitOptions} value={splitMode} onChange={setSplitMode} />
            {splitMode === 'custom' && <SplitEditor people={ctx.people} value={custom} onChange={setCustom} />}
          </>
        )}

        {!household && (
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

        {!editing && <InstallmentFields value={inst} onChange={setInst} today={today} cents={cents} />}

        {editing && expense?.installment && <p className={styles.hint}>{t('installments.editHint')}</p>}

        {conflict && (
          <p className={styles.notice} role="status">
            {t('conflict.current', { amount: formatBRL(conflict.amount_cents, i18n.language), description: conflict.description || '—' })}
          </p>
        )}
        {error != null && !conflict && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}

        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={pending} disabled={!valid}>
            {conflict ? t('conflict.apply') : editing ? t('composer.save') : t('composer.submit')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
