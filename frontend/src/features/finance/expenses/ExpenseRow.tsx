import { useTranslation } from 'react-i18next'
import { Avatar } from '@/design-system'
import type { Category, Expense } from '../api'
import { categoryName } from '../categories'
import { formatDay } from '../dates'
import { formatBRL } from '../money'
import { expenseActions } from '../permissions'
import { nameOf } from '../types'
import type { Ctx } from '../types'
import styles from '../finance.module.css'
import local from './Expenses.module.css'

/** Uma linha da lista: descrição (ou categoria), quem pagou, valor, data e os selos que contam de onde veio. */
export function ExpenseRow({
  expense,
  ctx,
  category,
  onOpen,
}: {
  expense: Expense
  ctx: Ctx
  category?: Category
  onOpen: (e: Expense) => void
}) {
  const { t, i18n } = useTranslation('finance')
  const catName = categoryName(t, category)
  const title = expense.description || catName
  const payer = nameOf(ctx, expense.paid_by)
  const shared = expenseActions(expense, ctx.me.id).sharedByPartner

  return (
    <li className={local.row} data-settled={expense.settled || undefined}>
      {expense.scope === 'household' ? <Avatar name={payer} className={local.avatar} /> : <span className={local.avatar} aria-hidden="true" />}
      <button type="button" className={styles.itemMain} onClick={() => onOpen(expense)} aria-label={t('row.open', { title })}>
        <span className={styles.itemTitle}>{title}</span>
        <span className={styles.itemMeta}>
          {expense.description && <span>{catName}</span>}
          <span>{formatDay(expense.spent_on, i18n.language)}</span>
          {expense.scope === 'household' && payer && <span>{t('row.paidBy', { name: expense.paid_by === ctx.me.id ? t('form.me') : payer })}</span>}
          {expense.installment && (
            <span className={styles.badge}>{t('row.installment', { no: expense.installment.no, total: expense.installment.total })}</span>
          )}
          {expense.settled && <span className={styles.badge}>{t('row.settled')}</span>}
          {expense.source !== 'manual' && <span className={styles.badge}>{t(`source.${expense.source}`)}</span>}
          {shared && <span className={styles.badge}>{t('row.sharedByPartner')}</span>}
          {expense.scope === 'personal' && !shared && expense.visibility === 'view' && <span className={styles.badge}>{t('row.visibleToPartner')}</span>}
        </span>
      </button>
      <span className={styles.money}>{formatBRL(expense.amount_cents, i18n.language)}</span>
    </li>
  )
}
