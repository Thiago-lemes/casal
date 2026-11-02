import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Avatar, Button } from '@/design-system'
import type { Task } from './api'
import { taskActions } from './permissions'
import type { Me, Person } from './types'
import styles from './TaskRow.module.css'

type Props = {
  task: Task
  me: Me
  /** todas as pessoas do casal, para mostrar nomes */
  people: Person[]
  onComplete: () => void
  onDecline: () => void
  onAssume: () => void
  onReassign: () => void
  onCancel: () => void
}

/** Uma tarefa. Mostra só as ações que a pessoa pode fazer (ver `taskActions`). */
export function TaskRow({ task, me, people, onComplete, onDecline, onAssume, onReassign, onCancel }: Props) {
  const { t, i18n } = useTranslation('tasks')
  const actions = taskActions(task, me.id)
  const nameOf = (id?: string | null) => people.find((p) => p.id === id)?.name ?? ''

  const fmt = (iso: string) =>
    new Intl.DateTimeFormat(i18n.language, {
      weekday: 'short',
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
      timeZone: me.timezone,
    }).format(new Date(iso))

  const late = task.status === 'open' && !!task.due_at && new Date(task.due_at) < new Date()
  const assigneeName = task.assignee_id ? nameOf(task.assignee_id) : ''
  const creator = task.created_by === me.id ? t('row.byYou') : t('row.by', { name: task.created_by_name })

  return (
    <li className={styles.row} data-status={task.status}>
      {task.status === 'done' ? (
        <span className={styles.done} role="img" aria-label={t('row.completed')}>
          <Check size={20} aria-hidden="true" />
        </span>
      ) : (
        <button
          type="button"
          className={styles.check}
          disabled={!actions.complete}
          onClick={onComplete}
          aria-label={t('row.complete', { title: task.title })}
        >
          <Check size={20} aria-hidden="true" />
        </button>
      )}

      <div className={styles.main}>
        <p className={styles.title}>{task.title}</p>

        <p className={styles.meta}>
          {task.assignee_id ? (
            <span className={styles.who}>
              <Avatar name={assigneeName || '?'} className={styles.avatar} />
              {assigneeName}
            </span>
          ) : (
            <span className={styles.shared}>{t('assignee.sharedTag')}</span>
          )}
          <span>{creator}</span>
          {task.repeat !== 'none' && (
            <span>{task.rotation ? t('repeat.withRotation', { repeat: t(`repeat.every.${task.repeat}`) }) : t(`repeat.every.${task.repeat}`)}</span>
          )}
          {task.due_at && (
            <span data-late={late || undefined}>{late ? `${t('due.late')} · ${fmt(task.due_at)}` : t('due.at', { when: fmt(task.due_at) })}</span>
          )}
        </p>

        {task.status === 'declined' && (
          <div className={styles.reason}>
            <p className={styles.reasonHead}>
              {t('row.declinedBy', { name: nameOf(task.assignee_id) || '?' })}
            </p>
            <p>{task.decline_reason}</p>
          </div>
        )}

        {task.status === 'done' && task.completed_at && (
          <p className={styles.doneBy}>
            {task.completed_by === me.id
              ? t('row.doneByYou', { when: fmt(task.completed_at) })
              : t('row.doneBy', { name: nameOf(task.completed_by) || '?', when: fmt(task.completed_at) })}
          </p>
        )}

        {(actions.decline || actions.assume || actions.reassign || actions.cancel) && (
          <div className={styles.actions}>
            {actions.decline && <Button onClick={onDecline}>{t('actions.decline')}</Button>}
            {actions.assume && <Button onClick={onAssume}>{t('actions.assume')}</Button>}
            {actions.reassign && (
              <Button variant="quiet" onClick={onReassign}>
                {t('actions.reassign')}
              </Button>
            )}
            {actions.cancel && (
              <Button variant="quiet" onClick={onCancel}>
                {t('actions.cancel')}
              </Button>
            )}
          </div>
        )}
      </div>
    </li>
  )
}
