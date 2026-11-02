import { ChevronLeft, ChevronRight, Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, EmptyState, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Expense } from '../api'
import { formatMonth, shiftMonth, todayIn, monthOf } from '../dates'
import { useCategories, useExpenses, useModuleGuard } from '../hooks'
import type { Ctx } from '../types'
import { ExpenseComposer } from './ExpenseComposer'
import type { Saved } from './ExpenseComposer'
import { ExpenseDialog } from './ExpenseDialog'
import { ExpenseRow } from './ExpenseRow'
import { GroupDialog } from './GroupDialog'
import styles from '../finance.module.css'
import local from './Expenses.module.css'

/** Aba Gastos: o mês, "Do casal" ou "Meus", e o botão de novo gasto. */
export function ExpenseList({ ctx }: { ctx: Ctx }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const [month, setMonth] = useState(() => monthOf(todayIn(ctx.me.timezone)))
  const [scope, setScope] = useState<'household' | 'mine'>('household')
  const [composing, setComposing] = useState(false)
  const [selected, setSelected] = useState<string>()
  const [editing, setEditing] = useState<Expense>()
  const [groupId, setGroupId] = useState<string>()
  const [notice, setNotice] = useState<string>()

  const query = useExpenses(month, scope)
  const categories = useCategories(true)
  const expenses = query.data?.pages.flatMap((p) => p.expenses) ?? []
  const current = expenses.find((e) => e.id === selected)
  const category = (id: string) => categories.data?.find((c) => c.id === id)

  useModuleGuard(query.error)

  function saved(info: Saved) {
    setNotice(info.fallback ? t('notice.splitFallback') : undefined)
  }

  return (
    <div className={styles.stack}>
      <div className={local.toolbar}>
        <div className={local.month}>
          <button type="button" className={local.nav} onClick={() => setMonth(shiftMonth(month, -1))} aria-label={t('month.previous')}>
            <ChevronLeft size={18} aria-hidden="true" />
          </button>
          <span aria-live="polite">{formatMonth(month, i18n.language)}</span>
          <button type="button" className={local.nav} onClick={() => setMonth(shiftMonth(month, 1))} aria-label={t('month.next')}>
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

      {notice && (
        <p className={styles.notice} role="status">
          {notice}
        </p>
      )}

      <div aria-live="polite">
        {query.isPending && (
          <div className={styles.skeletons}>
            <Skeleton />
            <Skeleton />
          </div>
        )}

        {query.isError && (
          <EmptyState
            title={t('loadError')}
            body={errorMessage(tc, query.error)}
            action={<Button onClick={() => void query.refetch()}>{tc('actions.tryAgain')}</Button>}
          />
        )}

        {query.isSuccess && expenses.length === 0 && (
          <EmptyState
            title={t('expenses.emptyTitle')}
            body={scope === 'mine' ? t('expenses.emptyMine') : t('expenses.emptyBody')}
            action={
              <Button variant="primary" onClick={() => setComposing(true)}>
                {t('expenses.new')}
              </Button>
            }
          />
        )}

        {expenses.length > 0 && (
          <div className={local.list}>
            <ul>
              {expenses.map((e) => (
                <ExpenseRow key={e.id} expense={e} ctx={ctx} category={category(e.category_id)} onOpen={(x) => setSelected(x.id)} />
              ))}
            </ul>
            {query.hasNextPage && (
              <div className={local.more}>
                <Button onClick={() => void query.fetchNextPage()} loading={query.isFetchingNextPage}>
                  {t('expenses.more')}
                </Button>
              </div>
            )}
          </div>
        )}
      </div>

      <Button variant="primary" className={local.fab} onClick={() => setComposing(true)}>
        <Plus size={18} aria-hidden="true" />
        {t('expenses.new')}
      </Button>

      {composing && <ExpenseComposer ctx={ctx} open onOpenChange={setComposing} onSaved={saved} />}

      {current && !editing && !groupId && (
        <ExpenseDialog
          expense={current}
          ctx={ctx}
          category={category(current.category_id)}
          onClose={() => setSelected(undefined)}
          onEdit={(e) => setEditing(e)}
          onGroup={setGroupId}
        />
      )}

      {editing && (
        <ExpenseComposer
          key={editing.id}
          ctx={ctx}
          open
          expense={editing}
          onOpenChange={(o) => {
            if (!o) {
              setEditing(undefined)
              setSelected(undefined)
            }
          }}
          onSaved={saved}
        />
      )}

      {groupId && <GroupDialog groupId={groupId} ctx={ctx} onClose={() => setGroupId(undefined)} />}
    </div>
  )
}
