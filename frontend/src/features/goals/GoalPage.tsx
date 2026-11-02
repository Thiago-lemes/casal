import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router-dom'
import { Button, Card, Dialog, DialogClose, EmptyState, Skeleton } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import { formatDay } from '@/shared/lib/dates'
import { isModuleDisabled } from '@/shared/modules/hooks'
import type { Goal } from './api'
import { FinancialPanel } from './financial/FinancialPanel'
import { GoalEditDialog } from './GoalEditDialog'
import { GoalProgress } from './GoalProgress'
import { HabitPanel } from './habit/HabitPanel'
import { useGoal, useGoalActions, useModuleGuard } from './hooks'
import { KIND_ICONS } from './kinds'
import { ModuleInvite } from './ModuleInvite'
import { goalActions } from './permissions'
import { ProjectPanel } from './project/ProjectPanel'
import { SharedPanel } from './shared/SharedPanel'
import type { Ctx, FinanceOptions } from './types'
import { nameOf } from './types'
import styles from './goals.module.css'

/** Detalhe de uma meta; o conteúdo muda por tipo. `404` é "não encontramos", nunca "é privada". */
export function GoalPage({ id, ctx, finance }: { id: string; ctx: Ctx; finance?: FinanceOptions }) {
  const { t, i18n } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const navigate = useNavigate()
  const query = useGoal(id)
  const { transition, remove } = useGoalActions()
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  useModuleGuard(query.error)

  const goal = query.data

  if (query.isPending) {
    return (
      <div className={styles.page}>
        <Skeleton height="8rem" />
        <Skeleton height="6rem" />
      </div>
    )
  }

  if (!goal) {
    const missing = query.error instanceof ApiError && query.error.status === 404
    return (
      <div className={styles.page}>
        {isModuleDisabled(query.error) ? (
          <ModuleInvite />
        ) : (
          <EmptyState
            title={missing ? t('notFound.title') : t('loadError')}
            body={missing ? t('notFound.body') : errorMessage(tc, query.error)}
            action={
              missing ? (
                <Link to="/goals">{t('notFound.back')}</Link>
              ) : (
                <Button onClick={() => void query.refetch()}>{tc('actions.tryAgain')}</Button>
              )
            }
          />
        )}
      </div>
    )
  }

  const actions = goalActions(goal)
  const view = goal.access === 'view'
  const Icon = KIND_ICONS[goal.kind]
  const owner = ctx.people.find((p) => p.id === goal.owner_id)
  const stepTasks = goal.kind === 'project' && goal.scope === 'household'

  const move = (action: Parameters<typeof transition.mutate>[0]['action']) => transition.mutate({ id: goal.id, action })

  return (
    <div className={styles.page}>
      <header className={styles.hero}>
        <div className={styles.heroText}>
          <Link to="/goals" className={styles.back}>
            {t('back')}
          </Link>
          <h1 className={styles.title}>{goal.title}</h1>
          <p className={styles.heroMeta}>
            <Icon size={18} aria-hidden="true" /> {t(`kind.${goal.kind}`)} · {t(`scope.${goal.scope}`)}
            {goal.status !== 'active' && ` · ${t(`status.${goal.status}`)}`}
          </p>
        </div>
      </header>

      {view && <p className={styles.notice}>{t('card.viewOnly', { name: owner?.name ?? t('card.partner') })}</p>}

      {goal.status === 'done' && (
        <p className={styles.notice} role="status">
          {t('detail.doneBy', {
            name: nameOf(ctx, goal.completed_by) || t('card.partner'),
            date: goal.completed_at ? new Intl.DateTimeFormat(i18n.language, { day: 'numeric', month: 'long', year: 'numeric' }).format(new Date(goal.completed_at)) : '',
          })}
        </p>
      )}

      {goal.status === 'active' && goal.progress.suggest_complete && actions.complete && (
        <div className={styles.notice} role="status">
          <p>{t(`suggest.${goal.kind}`)}</p>
          <Button variant="primary" loading={transition.isPending} onClick={() => move('complete')}>
            {t('actions.complete')}
          </Button>
        </div>
      )}

      <Card title={t('detail.progress')}>
        <div className={styles.stack}>
          <GoalProgress progress={goal.progress} label={goal.title} />
          {goal.due_on && (
            <p className={styles.hint}>{t('card.due', { date: formatDay(goal.due_on, i18n.language, { day: 'numeric', month: 'long', year: 'numeric' }) })}</p>
          )}
          {goal.notes && <p>{goal.notes}</p>}
        </div>
      </Card>

      {!view && goal.kind === 'habit' && <HabitPanel goal={goal} ctx={ctx} />}
      {!view && goal.kind === 'shared' && <SharedPanel goal={goal} ctx={ctx} />}
      {!view && goal.kind === 'financial' && <FinancialPanel goal={goal} me={ctx.me} finance={finance} />}
      {!view && goal.kind === 'project' && <ProjectPanel goal={goal} ctx={ctx} />}

      {!view && (
        <Card title={t('detail.manage')}>
          <div className={styles.stack}>
            <div className={styles.actions}>
              {actions.edit && <Button onClick={() => setEditing(true)}>{t('actions.edit')}</Button>}
              {actions.pause && <Button loading={transition.isPending} onClick={() => move('pause')}>{t('actions.pause')}</Button>}
              {actions.resume && (
                <Button loading={transition.isPending} onClick={() => move('resume')}>
                  {t(goal.status === 'done' ? 'actions.reopen' : 'actions.resume')}
                </Button>
              )}
              {actions.complete && !goal.progress.suggest_complete && (
                <Button loading={transition.isPending} onClick={() => move('complete')}>
                  {t('actions.complete')}
                </Button>
              )}
              {actions.archive && <Button loading={transition.isPending} onClick={() => move('archive')}>{t('actions.archive')}</Button>}
              {actions.unarchive && <Button loading={transition.isPending} onClick={() => move('unarchive')}>{t('actions.unarchive')}</Button>}
              {actions.delete && (
                <Button variant="danger" onClick={() => setDeleting(true)}>
                  {t('actions.delete')}
                </Button>
              )}
            </div>
            {transition.error != null && (
              <p className={styles.error} role="alert">
                {errorMessage(tc, transition.error)}
              </p>
            )}
          </div>
        </Card>
      )}

      {editing && <GoalEditDialog goal={goal} me={ctx.me} onClose={() => setEditing(false)} />}

      {deleting && (
        <DeleteDialog
          goal={goal}
          withTasks={stepTasks}
          pending={remove.isPending}
          error={remove.error}
          onCancel={() => setDeleting(false)}
          onConfirm={() => remove.mutate(goal.id, { onSuccess: () => navigate('/goals', { replace: true }) })}
        />
      )}
    </div>
  )
}

function DeleteDialog({ goal, withTasks, pending, error, onCancel, onConfirm }: { goal: Goal; withTasks: boolean; pending: boolean; error: unknown; onCancel: () => void; onConfirm: () => void }) {
  const { t } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  return (
    <Dialog open onOpenChange={(open) => !open && onCancel()} title={t('delete.title')}>
      <div className={styles.form}>
        <p>{t('delete.body', { title: goal.title })}</p>
        {withTasks && <p className={styles.hint}>{t('delete.tasks')}</p>}
        <p className={styles.hint}>{t('delete.final')}</p>
        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}
        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button variant="danger" loading={pending} onClick={onConfirm}>
            {t('delete.confirm')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
