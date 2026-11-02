import { useTranslation } from 'react-i18next'
import { Button } from '@/design-system'
import type { Bill } from '../api'
import { daysBetween, formatDay } from '@/shared/lib/dates'
import { formatBRL } from '@/shared/lib/money'
import { nameOf } from '../types'
import type { Ctx } from '../types'
import styles from '../finance.module.css'

/** Uma conta: vencidas em cor de alerta (com texto, não só cor), e o botão de pagar. */
export function BillRow({
  bill,
  ctx,
  today,
  onPay,
  onOpen,
}: {
  bill: Bill
  ctx: Ctx
  today: string
  onPay?: (b: Bill) => void
  onOpen?: (b: Bill) => void
}) {
  const { t, i18n } = useTranslation('finance')
  const paid = Boolean(bill.paid_at)
  const late = !paid && daysBetween(today, bill.due_on) < 0
  const dueToday = !paid && bill.due_on === today

  const main = (
    <>
      <span className={styles.itemTitle}>{bill.title}</span>
      <span className={styles.itemMeta}>
        <span>{formatDay(bill.due_on, i18n.language)}</span>
        {late && <span className={styles.badge} data-tone="alert">{t('bills.late')}</span>}
        {dueToday && <span className={styles.badge} data-tone="warn">{t('bills.today')}</span>}
        {ctx.partner && <span>{bill.responsible_id === ctx.me.id ? t('form.me') : nameOf(ctx, bill.responsible_id)}</span>}
        {bill.repeat === 'monthly' && <span className={styles.badge}>{t('bills.repeatMonthly')}</span>}
        {paid && <span className={styles.badge}>{t('bills.paid')}</span>}
        {bill.sync_pending && <span>{t('bills.syncPending')}</span>}
      </span>
    </>
  )

  return (
    <li className={styles.item}>
      {onOpen && !paid ? (
        <button type="button" className={styles.itemMain} onClick={() => onOpen(bill)} aria-label={t('bills.open', { title: bill.title })}>
          {main}
        </button>
      ) : (
        <span className={styles.itemMain}>{main}</span>
      )}
      <span style={{ display: 'grid', justifyItems: 'end', gap: 'var(--space-1)' }}>
        <span className={styles.money}>{bill.amount_cents ? formatBRL(bill.amount_cents, i18n.language) : t('bills.noAmount')}</span>
        {!paid && onPay && (
          <Button variant="primary" onClick={() => onPay(bill)} aria-label={t('bills.pay', { title: bill.title })}>
            {t('bills.payAction')}
          </Button>
        )}
      </span>
    </li>
  )
}
