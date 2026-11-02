import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Card, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Recurring } from '../api'
import { categoryName } from '../categories'
import { useCategories, useRecurring, useRecurringActions } from '../hooks'
import { formatBRL } from '@/shared/lib/money'
import { nameOf } from '../types'
import type { Ctx } from '../types'
import { RecurringForm } from './RecurringForm'
import styles from '../finance.module.css'

/** Os gastos fixos: valor, dia do mês, quem paga, categoria e estado. Criar, editar, pausar ou retomar, e apagar. */
export function RecurringList({ ctx }: { ctx: Ctx }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const list = useRecurring()
  const categories = useCategories(true)
  const actions = useRecurringActions()
  const [form, setForm] = useState<{ item?: Recurring } | null>(null)
  const [confirming, setConfirming] = useState<string>()
  const error = actions.pause.error ?? actions.resume.error ?? actions.remove.error

  return (
    <Card
      title={t('recurring.title')}
      aside={
        <Button variant="quiet" onClick={() => setForm({})}>
          {t('recurring.new')}
        </Button>
      }
    >
      <p className={styles.hint}>{t('recurring.help')}</p>
      {list.isPending && <Skeleton height="5rem" />}
      {list.isError && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, list.error)}
        </p>
      )}
      {list.isSuccess && list.data.length === 0 && <p className={styles.hint}>{t('recurring.empty')}</p>}
      <ul className={styles.list}>
        {(list.data ?? []).map((r) => (
          <li key={r.id} className={styles.item}>
            <span className={styles.itemMain}>
              <span className={styles.itemTitle}>{r.description || categoryName(t, categories.data?.find((c) => c.id === r.category_id))}</span>
              <span className={styles.itemMeta}>
                <span>{t('recurring.everyDay', { day: r.day_of_month })}</span>
                {r.scope === 'household' ? <span>{t('row.paidBy', { name: r.paid_by === ctx.me.id ? t('form.me') : nameOf(ctx, r.paid_by) })}</span> : <span className={styles.badge}>{t('scope.personal')}</span>}
                {!r.active && <span className={styles.badge}>{t('recurring.paused')}</span>}
              </span>
            </span>
            <span style={{ display: 'grid', justifyItems: 'end' }}>
              <span className={styles.money}>{formatBRL(r.amount_cents, i18n.language)}</span>
              {r.can_edit && (
                <span className={styles.segmented}>
                  <Button variant="quiet" onClick={() => setForm({ item: r })}>
                    {t('detail.edit')}
                  </Button>
                  <Button variant="quiet" onClick={() => (r.active ? actions.pause : actions.resume).mutate(r.id)}>
                    {r.active ? t('recurring.pause') : t('recurring.resume')}
                  </Button>
                  {confirming === r.id ? (
                    <Button variant="danger" loading={actions.remove.isPending} onClick={() => actions.remove.mutate(r.id, { onSuccess: () => setConfirming(undefined) })}>
                      {t('recurring.confirmDelete')}
                    </Button>
                  ) : (
                    <Button variant="quiet" onClick={() => setConfirming(r.id)}>
                      {t('detail.delete')}
                    </Button>
                  )}
                </span>
              )}
            </span>
          </li>
        ))}
      </ul>
      {error != null && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, error)}
        </p>
      )}
      {form && <RecurringForm key={form.item?.id ?? 'new'} ctx={ctx} recurring={form.item} onClose={() => setForm(null)} />}
    </Card>
  )
}
