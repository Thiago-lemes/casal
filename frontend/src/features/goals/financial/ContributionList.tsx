import { useTranslation } from 'react-i18next'
import { Button, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { formatDay } from '@/shared/lib/dates'
import { formatBRL } from '@/shared/lib/money'
import type { Goal } from '../api'
import { useContributionActions, useContributions } from '../hooks'
import type { Me } from '../types'
import styles from '../goals.module.css'

/** O histórico de aportes, mais recente primeiro. Só apaga quem registrou (ou o dono da meta). */
export function ContributionList({ goal, me, canRemove }: { goal: Goal; me: Me; canRemove: boolean }) {
  const { t, i18n } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const list = useContributions(goal.id)
  const { remove } = useContributionActions(goal.id)

  if (list.isPending) return <Skeleton height="6rem" />
  if (list.isError) {
    return (
      <div className={styles.form}>
        <p className={styles.error} role="alert">
          {errorMessage(tc, list.error)}
        </p>
        <div>
          <Button variant="quiet" onClick={() => void list.refetch()}>
            {tc('actions.tryAgain')}
          </Button>
        </div>
      </div>
    )
  }

  const items = list.data.pages.flatMap((p) => p.items)
  if (items.length === 0) return <p className={styles.hint}>{t('contribution.empty')}</p>

  return (
    <div className={styles.stack}>
      <ul className={styles.list}>
        {items.map((c) => {
          const withdraw = c.amount_cents < 0
          const mine = c.created_by === me.id || goal.owner_id === me.id
          return (
            <li key={c.id} className={styles.item}>
              <span className={styles.itemMain}>
                <span className={styles.money}>
                  {withdraw ? '−' : '+'}
                  {formatBRL(Math.abs(c.amount_cents), i18n.language)}
                </span>
                <span className={styles.meta}>
                  <span>{formatDay(c.made_on, i18n.language, { day: 'numeric', month: 'short', year: 'numeric' })}</span>
                  <span>· {withdraw ? t('contribution.withdrawBy', { name: c.created_by_name }) : t('contribution.addBy', { name: c.created_by_name })}</span>
                  {c.note && <span>· {c.note}</span>}
                </span>
              </span>
              {canRemove && mine && (
                <Button variant="quiet" loading={remove.isPending && remove.variables === c.id} onClick={() => remove.mutate(c.id)} aria-label={t('contribution.remove', { amount: formatBRL(Math.abs(c.amount_cents), i18n.language) })}>
                  {t('contribution.removeShort')}
                </Button>
              )}
            </li>
          )
        })}
      </ul>
      {remove.error != null && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, remove.error)}
        </p>
      )}
      {list.hasNextPage && (
        <div>
          <Button variant="quiet" loading={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}>
            {t('contribution.more')}
          </Button>
        </div>
      )}
    </div>
  )
}
