import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Button, Card, EmptyState, Skeleton, TextField } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import { formatDay, todayIn } from '../dates'
import { useBalance, useCategories, useModuleGuard } from '../hooks'
import { formatBRL } from '../money'
import { nameOf } from '../types'
import type { Ctx } from '../types'
import { standing } from './balance'
import { SettleDialog } from './SettleDialog'
import { SettlementHistory } from './SettlementHistory'
import styles from '../finance.module.css'
import local from './Balance.module.css'

/** Aba Saldo: quem deve a quem, quanto cada um pagou e os gastos que compõem o saldo. */
export function BalanceCard({ ctx }: { ctx: Ctx }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const today = todayIn(ctx.me.timezone)
  const [through, setThrough] = useState(today)
  const [settling, setSettling] = useState(false)
  const balance = useBalance(through, Boolean(ctx.partner))
  const categories = useCategories(true)
  useModuleGuard(balance.error)

  // casal de uma pessoa só: gastos funcionam, o acerto precisa de duas
  if (!ctx.partner || (balance.error instanceof ApiError && balance.error.is('single_member'))) {
    return (
      <EmptyState
        title={t('balance.singleTitle')}
        body={t('balance.singleBody')}
        action={
          <Link to="/home">
            <Button variant="primary">{t('balance.invite')}</Button>
          </Link>
        }
      />
    )
  }

  if (balance.isPending) return <Skeleton height="10rem" />
  if (balance.isError) {
    return (
      <EmptyState
        title={t('loadError')}
        body={errorMessage(tc, balance.error)}
        action={<Button onClick={() => void balance.refetch()}>{tc('actions.tryAgain')}</Button>}
      />
    )
  }

  const b = balance.data
  const s = standing(b, ctx.me.id)
  const lang = i18n.language
  const paidTotal = Object.values(b.paid_cents).reduce((n, c) => n + c, 0)

  return (
    <div className={styles.stack}>
      <Card title={t('balance.title')}>
        <p className={local.headline} aria-live="polite">
          {s.kind === 'even' && t('balance.even')}
          {s.kind === 'iOwe' && t('balance.iOwe', { amount: formatBRL(s.cents, lang), name: nameOf(ctx, s.to) })}
          {s.kind === 'owedToMe' && t('balance.owedToMe', { amount: formatBRL(s.cents, lang), name: nameOf(ctx, s.from) })}
        </p>
        <p className={styles.hint}>{t('balance.through', { date: formatDay(b.through, lang, { day: 'numeric', month: 'long' }) })}</p>

        {s.kind !== 'even' && (
          <div>
            <Button variant="primary" onClick={() => setSettling(true)}>
              {t('balance.settle')}
            </Button>
          </div>
        )}

        <details className={local.advanced}>
          <summary>{t('balance.advanced')}</summary>
          <TextField label={t('balance.throughLabel')} type="date" value={through} max={today} onChange={(e) => e.target.value && setThrough(e.target.value)} />
        </details>
      </Card>

      {paidTotal > 0 && (
        <Card title={t('balance.paidTitle')}>
          <div className={local.bar} role="img" aria-label={ctx.people.map((p) => `${p.name}: ${formatBRL(b.paid_cents[p.id] ?? 0, lang)}`).join(', ')}>
            {ctx.people.map((p, i) => (
              <span key={p.id} className={local.seg} data-person={i === 0 ? 'a' : 'b'} style={{ flexGrow: b.paid_cents[p.id] ?? 0 }} />
            ))}
          </div>
          <ul className={local.legend}>
            {ctx.people.map((p, i) => (
              <li key={p.id}>
                <span className={local.dot} data-person={i === 0 ? 'a' : 'b'} aria-hidden="true" />
                {p.id === ctx.me.id ? t('form.me') : p.name} <strong>{formatBRL(b.paid_cents[p.id] ?? 0, lang)}</strong>
              </li>
            ))}
          </ul>
        </Card>
      )}

      {b.expenses.length > 0 && (
        <Card title={t('balance.expenses')}>
          <ul className={styles.list}>
            {b.expenses.map((e) => {
              const c = categories.data?.find((x) => x.id === e.category_id)
              return (
                <li key={e.id} className={styles.item}>
                  <span className={styles.itemMain}>
                    <span className={styles.itemTitle}>{e.description || (c?.key ? t(`category.${c.key}`) : c?.name)}</span>
                    <span className={styles.itemMeta}>
                      <span>{formatDay(e.spent_on, lang)}</span>
                      <span>{t('row.paidBy', { name: e.paid_by === ctx.me.id ? t('form.me') : nameOf(ctx, e.paid_by) })}</span>
                    </span>
                  </span>
                  <span className={styles.money}>{formatBRL(e.amount_cents, lang)}</span>
                </li>
              )
            })}
          </ul>
        </Card>
      )}

      <SettlementHistory ctx={ctx} />

      {settling && <SettleDialog ctx={ctx} balance={b} onReload={() => balance.refetch()} onClose={() => setSettling(false)} />}
    </div>
  )
}
