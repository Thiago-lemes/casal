import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Category, Expense } from '../api'
import { categoryName } from '../categories'
import { formatDay } from '../dates'
import { useExpenseActions } from '../hooks'
import { formatBRL, splitLabel } from '../money'
import { expenseActions } from '../permissions'
import { nameOf } from '../types'
import type { Ctx } from '../types'
import styles from '../finance.module.css'
import local from './Expenses.module.css'

type Props = {
  expense: Expense
  ctx: Ctx
  category?: Category
  onClose: () => void
  onEdit: (e: Expense) => void
  onGroup: (groupId: string) => void
}

/** O detalhe de um gasto e as ações que `expenseActions` permitir. O servidor continua sendo a autoridade. */
export function ExpenseDialog({ expense, ctx, category, onClose, onEdit, onGroup }: Props) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const actions = useExpenseActions()
  const can = expenseActions(expense, ctx.me.id)
  const [confirming, setConfirming] = useState(false)
  const lang = i18n.language

  function remove() {
    actions.remove.mutate(expense.id, { onSuccess: onClose })
  }

  function toggleVisibility() {
    actions.update.mutate({
      id: expense.id,
      body: {
        scope: 'personal',
        visibility: expense.visibility === 'view' ? 'private' : 'view',
        amount_cents: expense.amount_cents,
        category_id: expense.category_id,
        spent_on: expense.spent_on,
        description: expense.description,
        version: expense.version,
      },
    })
  }

  const error = actions.remove.error ?? actions.update.error

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={expense.description || categoryName(t, category)}>
      <div className={styles.form}>
        <p className={local.total}>{formatBRL(expense.amount_cents, lang)}</p>

        <dl className={local.detail}>
          <dt>{t('form.category')}</dt>
          <dd>{categoryName(t, category)}</dd>
          <dt>{t('form.date')}</dt>
          <dd>{formatDay(expense.spent_on, lang, { day: 'numeric', month: 'long', year: 'numeric' })}</dd>
          {expense.scope === 'household' && (
            <>
              <dt>{t('form.paidBy')}</dt>
              <dd>{expense.paid_by === ctx.me.id ? t('form.me') : nameOf(ctx, expense.paid_by)}</dd>
            </>
          )}
          {expense.split && (
            <>
              <dt>{t('split.legend')}</dt>
              <dd>
                {ctx.people.map((p) => `${p.id === ctx.me.id ? t('form.me') : p.name} ${splitLabel(expense.split?.[p.id] ?? 0, lang)}`).join(' · ')}
              </dd>
            </>
          )}
          {expense.scope === 'personal' && (
            <>
              <dt>{t('visibility.legend')}</dt>
              <dd>{can.sharedByPartner ? t('row.sharedByPartner') : t(`visibility.${expense.visibility === 'view' ? 'view' : 'private'}`)}</dd>
            </>
          )}
          {expense.installment && (
            <>
              <dt>{t('installments.toggle')}</dt>
              <dd>{t('row.installment', { no: expense.installment.no, total: expense.installment.total })}</dd>
            </>
          )}
        </dl>

        {expense.settled && <p className={styles.hint}>{t('detail.settledHint')}</p>}

        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}

        {confirming ? (
          <div className={styles.form}>
            <p>{t('detail.confirmDelete')}</p>
            <div className={styles.footer}>
              <Button variant="quiet" onClick={() => setConfirming(false)}>
                {tc('actions.cancel')}
              </Button>
              <Button variant="danger" loading={actions.remove.isPending} onClick={remove}>
                {t('detail.delete')}
              </Button>
            </div>
          </div>
        ) : (
          <div className={styles.footer}>
            {expense.installment && (
              <Button variant="quiet" onClick={() => onGroup(expense.installment!.group_id)}>
                {t('detail.wholePurchase')}
              </Button>
            )}
            {can.changeVisibility && (
              <Button variant="quiet" loading={actions.update.isPending} onClick={toggleVisibility}>
                {expense.visibility === 'view' ? t('detail.makePrivate') : t('detail.sharePartner')}
              </Button>
            )}
            {can.remove && (
              <Button variant="quiet" onClick={() => setConfirming(true)}>
                {t('detail.delete')}
              </Button>
            )}
            {can.edit && (
              <Button variant="primary" onClick={() => onEdit(expense)}>
                {t('detail.edit')}
              </Button>
            )}
            <DialogClose asChild>
              <Button variant="quiet">{tc('actions.close')}</Button>
            </DialogClose>
          </div>
        )}
      </div>
    </Dialog>
  )
}
