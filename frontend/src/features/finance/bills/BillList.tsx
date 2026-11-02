import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, EmptyState, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Bill, PayResult } from '../api'
import { formatDay, todayIn } from '../dates'
import { useBillActions, useBills, useModuleGuard } from '../hooks'
import type { Ctx } from '../types'
import { BillForm } from './BillForm'
import { BillRow } from './BillRow'
import { PayDialog } from './PayDialog'
import styles from '../finance.module.css'
import local from '../expenses/Expenses.module.css'

/** Aba Contas: as abertas (vencidas primeiro) e as pagas. */
export function BillList({ ctx }: { ctx: Ctx }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const [status, setStatus] = useState<'open' | 'paid'>('open')
  const list = useBills(status)
  const actions = useBillActions()
  const [form, setForm] = useState<{ bill?: Bill } | null>(null)
  const [paying, setPaying] = useState<Bill>()
  const [notice, setNotice] = useState<string>()
  const today = todayIn(ctx.me.timezone)
  useModuleGuard(list.error)

  const bills = [...(list.data ?? [])].sort((a, b) => (status === 'open' ? a.due_on.localeCompare(b.due_on) : (b.paid_at ?? '').localeCompare(a.paid_at ?? '')))

  function paid(res: PayResult) {
    const parts = [res.split_fallback ? t('notice.splitFallback') : undefined, res.next_bill ? t('notice.nextBill', { date: formatDay(res.next_bill.due_on, i18n.language, { day: 'numeric', month: 'long' }) }) : undefined]
    setNotice(parts.filter(Boolean).join(' ') || t('notice.billPaid'))
  }

  return (
    <div className={styles.stack}>
      <div className={local.toolbar}>
        <ChoiceGroup
          legend={t('bills.status')}
          options={[
            { value: 'open', label: t('bills.open_tab') },
            { value: 'paid', label: t('bills.paid_tab') },
          ]}
          value={status}
          onChange={setStatus}
        />
      </div>

      {notice && (
        <p className={styles.notice} role="status">
          {notice}
        </p>
      )}

      <div aria-live="polite">
        {list.isPending && <Skeleton height="6rem" />}
        {list.isError && (
          <EmptyState
            title={t('loadError')}
            body={errorMessage(tc, list.error)}
            action={<Button onClick={() => void list.refetch()}>{tc('actions.tryAgain')}</Button>}
          />
        )}
        {list.isSuccess && bills.length === 0 && (
          <EmptyState title={status === 'open' ? t('bills.emptyTitle') : t('bills.emptyPaid')} body={status === 'open' ? t('bills.emptyBody') : undefined} />
        )}
        {bills.length > 0 && (
          <div className={local.list}>
            <ul>
              {bills.map((b) => (
                <BillRow key={b.id} bill={b} ctx={ctx} today={today} onPay={setPaying} onOpen={(x) => setForm({ bill: x })} />
              ))}
            </ul>
          </div>
        )}
      </div>

      {actions.remove.error != null && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, actions.remove.error)}
        </p>
      )}

      <Button variant="primary" className={local.fab} onClick={() => setForm({})}>
        <Plus size={18} aria-hidden="true" />
        {t('bills.new')}
      </Button>

      {form && (
        <BillForm
          key={form.bill?.id ?? 'new'}
          ctx={ctx}
          bill={form.bill}
          onClose={() => setForm(null)}
          onSaved={(b) => setNotice(b.sync_pending ? t('notice.syncPending') : undefined)}
        />
      )}
      {paying && <PayDialog ctx={ctx} bill={paying} onClose={() => setPaying(undefined)} onPaid={paid} />}
    </div>
  )
}
