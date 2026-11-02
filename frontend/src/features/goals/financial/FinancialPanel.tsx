import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Button, Card } from '@/design-system'
import type { Goal } from '../api'
import { goalActions } from '../permissions'
import type { FinanceOptions, Me } from '../types'
import { ContributionDialog } from './ContributionDialog'
import { ContributionList } from './ContributionList'
import { PaceNote } from './PaceNote'
import styles from '../goals.module.css'

/**
 * Meta financeira. Com aportes: o histórico e os botões Aportar e Retirar (só um registro da meta, não mexe em Finanças).
 * Com economia do orçamento: o valor é calculado pelo servidor, sem botão de aporte.
 */
export function FinancialPanel({ goal, me, finance }: { goal: Goal; me: Me; finance?: FinanceOptions }) {
  const { t } = useTranslation('goals')
  const [dialog, setDialog] = useState<'add' | 'withdraw'>()
  const financial = goal.financial
  if (!financial) return null
  const actions = goalActions(goal)
  const budget = financial.source === 'budget_savings'
  const category = finance?.categories.find((c) => c.id === financial.category_id)?.name

  return (
    <>
      <Card title={t(budget ? 'financial.budgetTitle' : 'financial.title')}>
        <div className={styles.stack}>
          <PaceNote progress={goal.progress} />

          {budget && (
            <>
              <p className={styles.hint}>{category ? t('financial.budgetOf', { category }) : t('financial.budgetHint')}</p>
              {goal.progress.unavailable ? (
                <p className={styles.notice}>
                  {t('financial.unavailable')} <Link to="/finance">{t('financial.unavailableLink')}</Link>
                </p>
              ) : (
                goal.progress.provisional && <p className={styles.hint}>{t('financial.provisional')}</p>
              )}
            </>
          )}

          {!budget && actions.contribute && (
            <div className={styles.actions}>
              <Button variant="primary" onClick={() => setDialog('add')}>
                {t('contribution.addAction')}
              </Button>
              <Button onClick={() => setDialog('withdraw')}>{t('contribution.withdrawAction')}</Button>
            </div>
          )}
        </div>
      </Card>

      {!budget && (
        <Card title={t('contribution.title')}>
          <ContributionList goal={goal} me={me} canRemove={actions.contribute} />
        </Card>
      )}

      {dialog && <ContributionDialog goal={goal} me={me} mode={dialog} onClose={() => setDialog(undefined)} />}
    </>
  )
}
