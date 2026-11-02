import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, EmptyState, Skeleton, Tabs } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useModuleEnabled } from '@/shared/modules/hooks'
import type { Goal, GoalQuery, Kind, Status } from './api'
import { GoalCard } from './GoalCard'
import { GoalWizard } from './create/GoalWizard'
import { useGoals, useModuleGuard } from './hooks'
import { ModuleInvite } from './ModuleInvite'
import type { Ctx, FinanceOptions } from './types'
import styles from './goals.module.css'

type Tab = 'household' | 'mine'
const KINDS: Kind[] = ['habit', 'shared', 'financial', 'project']
const STATUSES: Status[] = ['active', 'paused', 'done', 'archived']

/** Separa as pessoais (minhas) das que o parceiro compartilhou para leitura. Não ordena nem compara pessoas. */
export function splitMine(goals: Goal[]): { own: Goal[]; shared: Goal[] } {
  const personal = goals.filter((g) => g.scope === 'personal')
  return { own: personal.filter((g) => g.access === 'full'), shared: personal.filter((g) => g.access === 'view') }
}

export function GoalsPage({ ctx, finance }: { ctx: Ctx; finance?: FinanceOptions }) {
  const { t } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const mod = useModuleEnabled('goals')
  const [tab, setTab] = useState<Tab>('household')
  const [kind, setKind] = useState<Kind | ''>('')
  const [status, setStatus] = useState<Status>('active')
  const kindId = useId()
  const statusId = useId()

  // "Minhas" pede tudo que a pessoa vê e separa no cliente: as pessoais do parceiro (`view`) só vêm sem `scope`
  const query: GoalQuery = { scope: tab === 'household' ? 'household' : undefined, kind: kind || undefined, status }
  const list = useGoals(query, mod.on)
  useModuleGuard(list.error)

  const creating = params.get('new') === '1'
  const setCreating = (open: boolean) => setParams(open ? { new: '1' } : {}, { replace: true })

  const goals = list.data ?? []
  const { own, shared } = splitMine(goals)
  const shown = tab === 'household' ? goals : own
  const nothing = list.isSuccess && shown.length === 0 && (tab === 'household' || shared.length === 0)
  const firstTime = nothing && !kind && status === 'active'

  return (
    <div className={styles.page}>
      <header className={styles.hero}>
        <div className={styles.heroText}>
          <h1 className={styles.title}>{t('title')}</h1>
        </div>
        {mod.on && (
          <Button variant="secondary" onClick={() => setCreating(true)}>
            {t('new')}
          </Button>
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

      {mod.on && (
        <Tabs
          label={t('title')}
          tabs={[
            { value: 'household', label: t('tabs.household') },
            { value: 'mine', label: t('tabs.mine') },
          ]}
          value={tab}
          onChange={setTab}
        >
          <div className={styles.stack}>
            <div className={styles.filters}>
              <div className={styles.field}>
                <label htmlFor={kindId} className={styles.label}>
                  {t('filters.kind')}
                </label>
                <select id={kindId} className={styles.select} value={kind} onChange={(e) => setKind(e.target.value as Kind | '')}>
                  <option value="">{t('filters.allKinds')}</option>
                  {KINDS.map((k) => (
                    <option key={k} value={k}>
                      {t(`kind.${k}`)}
                    </option>
                  ))}
                </select>
              </div>
              <div className={styles.field}>
                <label htmlFor={statusId} className={styles.label}>
                  {t('filters.status')}
                </label>
                <select id={statusId} className={styles.select} value={status} onChange={(e) => setStatus(e.target.value as Status)}>
                  {STATUSES.map((s) => (
                    <option key={s} value={s}>
                      {t(`filters.${s}`)}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            {list.isPending && (
              <div className={styles.skeletons}>
                <Skeleton />
                <Skeleton />
              </div>
            )}

            {list.isError && !list.isPending && (
              <EmptyState
                title={t('loadError')}
                body={errorMessage(tc, list.error)}
                action={<Button onClick={() => void list.refetch()}>{tc('actions.tryAgain')}</Button>}
              />
            )}

            {firstTime && (
              <EmptyState
                title={t('empty.title')}
                body={t('empty.body')}
                action={
                  <Button variant="primary" onClick={() => setCreating(true)}>
                    {t('empty.action')}
                  </Button>
                }
              />
            )}
            {nothing && !firstTime && <EmptyState title={t('empty.filtered')} />}

            {list.isSuccess && shown.length > 0 && (
              <ul className={styles.cards}>
                {shown.map((g) => (
                  <li key={g.id}>
                    <GoalCard goal={g} people={ctx.people} />
                  </li>
                ))}
              </ul>
            )}

            {tab === 'mine' && list.isSuccess && shared.length > 0 && (
              <section className={styles.stack} aria-label={t('sharedSection')}>
                <h2 className={styles.sectionTitle}>{t('sharedSection')}</h2>
                <ul className={styles.cards}>
                  {shared.map((g) => (
                    <li key={g.id}>
                      <GoalCard goal={g} people={ctx.people} />
                    </li>
                  ))}
                </ul>
              </section>
            )}
          </div>
        </Tabs>
      )}

      {mod.on && creating && (
        <GoalWizard open onOpenChange={setCreating} me={ctx.me} finance={finance} onCreated={(g) => navigate(`/goals/${g.id}`)} />
      )}
    </div>
  )
}
