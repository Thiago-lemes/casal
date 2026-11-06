import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Card, ChoiceGroup, EmptyState, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { categoryName } from '../categories'
import { formatMonth, monthOf, shiftMonth, todayIn } from '@/shared/lib/dates'
import { useCards, useCategories, useModuleGuard, useSummary } from '../hooks'
import { formatBRL } from '@/shared/lib/money'
import { nameOf } from '../types'
import type { Ctx } from '../types'
import { BudgetBar } from './BudgetBar'
import { compare } from './summary'
import styles from '../finance.module.css'
import local from './Summary.module.css'
import nav from '../expenses/Expenses.module.css'

/** Aba Resumo: total do mês, comparação em tom neutro, barras por categoria e quanto cada um pagou. */
export function MonthSummary({ ctx }: { ctx: Ctx }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const [month, setMonth] = useState(() => monthOf(todayIn(ctx.me.timezone)))
  const [scope, setScope] = useState<'household' | 'mine'>('household')
  const summary = useSummary(month, scope)
  const categories = useCategories(true)
  const cards = useCards(true)
  useModuleGuard(summary.error)
  const lang = i18n.language

  const s = summary.data
  const rows = [...(s?.categories ?? [])].filter((c) => c.total_cents > 0 || c.limit_cents !== undefined).sort((a, b) => b.total_cents - a.total_cents)
  const max = Math.max(0, ...rows.map((r) => r.total_cents))
  const cmp = s ? compare(s.total_cents, s.previous_total_cents) : undefined
  const prevName = formatMonth(shiftMonth(month, -1), lang).split(' ')[0]

  return (
    <div className={styles.stack}>
      <div className={nav.toolbar}>
        <div className={nav.month}>
          <button type="button" className={nav.nav} onClick={() => setMonth(shiftMonth(month, -1))} aria-label={t('month.previous')}>
            <ChevronLeft size={18} aria-hidden="true" />
          </button>
          <span aria-live="polite">{formatMonth(month, lang)}</span>
          <button type="button" className={nav.nav} onClick={() => setMonth(shiftMonth(month, 1))} aria-label={t('month.next')}>
            <ChevronRight size={18} aria-hidden="true" />
          </button>
        </div>
        <ChoiceGroup
          legend={t('scope.legend')}
          options={[
            { value: 'household', label: t('scope.household') },
            { value: 'mine', label: t('scope.mine') },
          ]}
          value={scope}
          onChange={setScope}
        />
      </div>

      {summary.isPending && <Skeleton height="10rem" />}
      {summary.isError && (
        <EmptyState
          title={t('loadError')}
          body={errorMessage(tc, summary.error)}
          action={<Button onClick={() => void summary.refetch()}>{tc('actions.tryAgain')}</Button>}
        />
      )}

      {s && cmp && (
        <>
          <Card title={scope === 'mine' ? t('summary.totalMine') : t('summary.total')}>
            <p className={local.total}>{formatBRL(s.total_cents, lang)}</p>
            <p className={local.compare}>
              {cmp.kind === 'same' && t('summary.same', { month: prevName })}
              {cmp.kind !== 'same' &&
                t(cmp.percent === null ? `summary.${cmp.kind}NoPercent` : `summary.${cmp.kind}`, {
                  amount: formatBRL(cmp.diff, lang),
                  percent: cmp.percent,
                  month: prevName,
                })}
            </p>
            {scope === 'mine' && <p className={styles.hint}>{t('summary.mineHint')}</p>}
          </Card>

          {s.total_cents === 0 && rows.length === 0 ? (
            <EmptyState title={t('summary.emptyTitle')} body={t('summary.emptyBody')} />
          ) : (
            <Card title={t('summary.byCategory')}>
              <ul className={local.bars}>
                {rows.map((r) => (
                  <BudgetBar
                    key={r.category_id}
                    name={categoryName(t, categories.data?.find((c) => c.id === r.category_id))}
                    totalCents={r.total_cents}
                    limitCents={r.limit_cents}
                    usedPercent={r.used_percent}
                    maxCents={max}
                  />
                ))}
              </ul>
              {scope === 'household' && <p className={styles.hint}>{t('summary.installmentsHint')}</p>}
            </Card>
          )}

          {(s.cards.length > 0 || (cards.data ?? []).length > 0) && (
            <Card title={t('summary.byCard')}>
              <ul className={styles.list}>
                {s.cards.map((c) => (
                  <li key={c.card_id} className={styles.item}>
                    <span className={styles.itemTitle}>{cards.data?.find((x) => x.id === c.card_id)?.name ?? ''}</span>
                    <span className={styles.money}>{formatBRL(c.total_cents, lang)}</span>
                  </li>
                ))}
                {s.without_card_cents > 0 && (
                  <li className={styles.item}>
                    <span className={styles.itemTitle}>{t('summary.withoutCard')}</span>
                    <span className={styles.money}>{formatBRL(s.without_card_cents, lang)}</span>
                  </li>
                )}
              </ul>
              {scope === 'household' && <p className={styles.hint}>{t('summary.byCardHint')}</p>}
            </Card>
          )}

          {scope === 'household' && s.paid_by && Object.keys(s.paid_by).length > 0 && (
            <Card title={t('summary.paidBy')}>
              <ul className={styles.list}>
                {ctx.people.map((p) => (
                  <li key={p.id} className={styles.item}>
                    <span className={styles.itemTitle}>{p.id === ctx.me.id ? t('form.me') : nameOf(ctx, p.id)}</span>
                    <span className={styles.money}>{formatBRL(s.paid_by?.[p.id] ?? 0, lang)}</span>
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </>
      )}
    </div>
  )
}
