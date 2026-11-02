import { Settings } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, EmptyState, Skeleton, Tabs } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useModuleEnabled } from '@/shared/modules/hooks'
import { BalanceCard } from './balance/BalanceCard'
import { BillList } from './bills/BillList'
import { ExpenseList } from './expenses/ExpenseList'
import { ModuleInvite } from './ModuleInvite'
import { SettingsView } from './settings/SettingsView'
import { MonthSummary } from './summary/MonthSummary'
import type { Me, Person } from './types'
import styles from './FinancePage.module.css'

type Tab = 'expenses' | 'balance' | 'bills' | 'summary'

/** `people` traz o casal inteiro (inclui quem está logado); a tela que compõe as features entrega isso. */
export function FinancePage({ me, people }: { me: Me; people: Person[] }) {
  const { t } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const mod = useModuleEnabled('finance')
  const [tab, setTab] = useState<Tab>('expenses')
  const [settings, setSettings] = useState(false)

  const ctx = { me, people, partner: people.find((p) => p.id !== me.id) }

  return (
    <div className={styles.page}>
      <header className={styles.hero}>
        <h1 className={styles.title}>{t('title')}</h1>
        {mod.on && (
          <button type="button" className={styles.gear} onClick={() => setSettings(!settings)} aria-pressed={settings} aria-label={t('settings.open')}>
            <Settings size={22} aria-hidden="true" />
          </button>
        )}
      </header>

      {mod.isPending && <Skeleton height="8rem" />}
      {mod.isSuccess && !mod.on && <ModuleInvite />}
      {mod.isError && (
        <EmptyState
          title={t('loadError')}
          body={errorMessage(tc, mod.error)}
          action={<Button onClick={() => void mod.refetch()}>{tc('actions.tryAgain')}</Button>}
        />
      )}

      {mod.on &&
        (settings ? (
          <SettingsView ctx={ctx} onBack={() => setSettings(false)} />
        ) : (
          <Tabs
            label={t('title')}
            tabs={[
              { value: 'expenses', label: t('tabs.expenses') },
              { value: 'balance', label: t('tabs.balance') },
              { value: 'bills', label: t('tabs.bills') },
              { value: 'summary', label: t('tabs.summary') },
            ]}
            value={tab}
            onChange={setTab}
          >
            {tab === 'expenses' && <ExpenseList ctx={ctx} />}
            {tab === 'balance' && <BalanceCard ctx={ctx} />}
            {tab === 'bills' && <BillList ctx={ctx} />}
            {tab === 'summary' && <MonthSummary ctx={ctx} />}
          </Tabs>
        ))}
    </div>
  )
}
