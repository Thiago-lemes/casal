import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Card, Skeleton, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { categoryName, selectable } from '../categories'
import { useBudgetActions, useBudgets, useCategories } from '../hooks'
import { centsToInput, formatBRL, MAX_CENTS, parseMoney } from '../money'
import styles from '../finance.module.css'

/** O limite mensal por categoria do casal. Gasto pessoal não conta; não há orçamento pessoal nesta fase. */
export function BudgetEditor() {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const budgets = useBudgets()
  const categories = useCategories(false)
  const actions = useBudgetActions()
  const [drafts, setDrafts] = useState<Record<string, string>>({})

  const list = selectable(categories.data ?? [])
  const limit = (id: string) => budgets.data?.find((b) => b.category_id === id)?.limit_cents
  const error = actions.save.error ?? actions.remove.error

  return (
    <Card title={t('budget.title')}>
      <p className={styles.hint}>{t('budget.help')}</p>
      {(budgets.isPending || categories.isPending) && <Skeleton height="6rem" />}
      <ul className={styles.list}>
        {list.map((c) => {
          const current = limit(c.id)
          const draft = drafts[c.id] ?? (current ? centsToInput(current) : '')
          const cents = parseMoney(draft)
          const bad = draft.trim() !== '' && (cents === null || cents <= 0 || cents > MAX_CENTS)
          return (
            <li key={c.id} className={styles.item} style={{ gridTemplateColumns: 'minmax(0, 1fr) auto' }}>
              <form
                className={styles.form}
                onSubmit={(e) => {
                  e.preventDefault()
                  if (cents && !bad) actions.save.mutate({ category_id: c.id, limit_cents: cents })
                }}
                noValidate
              >
                <TextField
                  label={categoryName(t, c)}
                  inputMode="decimal"
                  placeholder={t('budget.noLimit')}
                  value={draft}
                  onChange={(e) => setDrafts({ ...drafts, [c.id]: e.target.value })}
                  error={bad ? t('form.amountInvalid') : undefined}
                  hint={current ? t('budget.current', { limit: formatBRL(current, i18n.language) }) : undefined}
                />
                <div className={styles.footer}>
                  {current !== undefined && (
                    <Button variant="quiet" onClick={() => actions.remove.mutate(c.id, { onSuccess: () => setDrafts({ ...drafts, [c.id]: '' }) })} aria-label={t('budget.removeOf', { name: categoryName(t, c) })}>
                      {t('budget.remove')}
                    </Button>
                  )}
                  <Button type="submit" disabled={!cents || bad || cents === current} aria-label={t('budget.saveOf', { name: categoryName(t, c) })}>
                    {t('budget.save')}
                  </Button>
                </div>
              </form>
            </li>
          )
        })}
      </ul>
      {error != null && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, error)}
        </p>
      )}
    </Card>
  )
}
