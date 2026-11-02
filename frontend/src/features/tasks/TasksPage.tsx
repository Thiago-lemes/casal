import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, EmptyState, Skeleton, Tabs } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import type { ListStatus, Task } from './api'
import { DeclineDialog } from './DeclineDialog'
import { useTaskActions, useTasks } from './hooks'
import { sortDone, sortOpen, toAssignment } from './permissions'
import type { Choice } from './permissions'
import { ReassignDialog } from './ReassignDialog'
import { TaskComposer } from './TaskComposer'
import { TaskRow } from './TaskRow'
import type { Me, Person } from './types'
import styles from './TasksPage.module.css'

const TABS: ListStatus[] = ['open', 'declined', 'done']

/** `people` traz o casal inteiro (inclui quem está logado); a tela que compõe as features entrega isso. */
export function TasksPage({ me, people }: { me: Me; people: Person[] }) {
  const { t } = useTranslation('tasks')
  const { t: tc } = useTranslation('common')
  const actions = useTaskActions()
  const [tab, setTab] = useState<ListStatus>('open')
  const [composing, setComposing] = useState(false)
  const [declining, setDeclining] = useState<Task | null>(null)
  const [reassigning, setReassigning] = useState<Task | null>(null)
  const [notice, setNotice] = useState<string>()

  const partner = people.find((p) => p.id !== me.id)
  const open = useTasks('open')
  const list = useTasks(tab, tab !== 'open')
  const current = tab === 'open' ? open : list

  const tasks = !current.data ? [] : tab === 'open' ? sortOpen(current.data) : tab === 'done' ? sortDone(current.data) : current.data

  /** Uma tarefa que sumiu ou mudou de dono (`task_not_found`) vira um aviso único e uma lista nova. */
  function onActionError(err: unknown) {
    if (err instanceof ApiError && err.is('task_not_found')) {
      setNotice(t('unavailable'))
      void actions.refresh()
    } else {
      setNotice(errorMessage(tc, err))
    }
  }

  function act<V>(mutate: (vars: V, o: { onError: (e: unknown) => void; onSuccess?: () => void }) => void, vars: V, onSuccess?: () => void) {
    setNotice(undefined)
    mutate(vars, { onError: onActionError, onSuccess })
  }

  const assign = (task: Task, choice: Choice, done?: () => void) =>
    act(actions.assign.mutate, { id: task.id, to: toAssignment(choice, me.id, partner?.id) }, done)

  const openCount = open.data?.length ?? 0

  return (
    <div className={styles.page}>
      <header className={styles.hero}>
        <div>
          <h1 className={styles.title}>{t('title')}</h1>
          {open.isSuccess && <p className={styles.count}>{t('openCount', { count: openCount })}</p>}
        </div>
        <Button variant="primary" className={styles.cta} onClick={() => setComposing(true)}>
          {t('new')}
        </Button>
      </header>

      <Tabs
        label={t('tabs.label')}
        value={tab}
        onChange={(v) => {
          setNotice(undefined)
          setTab(v)
        }}
        tabs={TABS.map((v) => ({ value: v, label: t(`tabs.${v}`) }))}
      >
        {notice && (
          <p className={styles.notice} role="alert">
            {notice}
          </p>
        )}

        <div aria-live="polite">
          {current.isPending && (
            <div className={styles.skeletons}>
              <Skeleton />
              <Skeleton />
            </div>
          )}

          {current.isError && (
            <EmptyState
              title={t('loadError')}
              body={errorMessage(tc, current.error)}
              action={<Button onClick={() => void current.refetch()}>{tc('actions.tryAgain')}</Button>}
            />
          )}

          {current.isSuccess && tasks.length === 0 && (
            <EmptyState
              title={t(`empty.${tab}.title`)}
              body={t(`empty.${tab}.body`)}
              action={
                tab === 'open' ? (
                  <Button variant="primary" onClick={() => setComposing(true)}>
                    {t('new')}
                  </Button>
                ) : undefined
              }
            />
          )}

          {current.isSuccess && tasks.length > 0 && (
            <ul className={styles.list}>
              {tasks.map((task) => (
                <TaskRow
                  key={task.id}
                  task={task}
                  me={me}
                  people={people}
                  onComplete={() => act(actions.complete.mutate, task.id)}
                  onDecline={() => setDeclining(task)}
                  onAssume={() => assign(task, 'me')}
                  onReassign={() => setReassigning(task)}
                  onCancel={() => act(actions.cancel.mutate, task.id)}
                />
              ))}
            </ul>
          )}
        </div>
      </Tabs>

      <TaskComposer open={composing} onOpenChange={setComposing} me={me} partner={partner} actions={actions} />

      <DeclineDialog
        key={declining?.id}
        open={declining !== null}
        onOpenChange={(o) => !o && setDeclining(null)}
        pending={actions.decline.isPending}
        error={actions.decline.error}
        onSubmit={(reason) => {
          if (!declining) return
          actions.decline.mutate(
            { id: declining.id, reason },
            {
              onSuccess: () => setDeclining(null),
              onError: (err) => {
                if (err instanceof ApiError && err.is('task_not_found')) {
                  setDeclining(null)
                  onActionError(err)
                }
              },
            },
          )
        }}
      />

      <ReassignDialog
        key={reassigning?.id}
        open={reassigning !== null}
        onOpenChange={(o) => !o && setReassigning(null)}
        partner={partner}
        pending={actions.assign.isPending}
        error={actions.assign.error}
        onSubmit={(choice) => reassigning && assign(reassigning, choice, () => setReassigning(null))}
      />
    </div>
  )
}
