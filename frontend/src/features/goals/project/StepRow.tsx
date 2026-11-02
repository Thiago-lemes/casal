import { ArrowDown, ArrowUp } from 'lucide-react'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/design-system'
import { formatDay } from '@/shared/lib/dates'
import type { Step } from '../api'
import type { Ctx } from '../types'
import { nameOf } from '../types'
import styles from '../goals.module.css'

type Props = {
  step: Step
  ctx: Ctx
  /** quem pode mexer nas etapas (meta aberta e permissão de edição) */
  editable: boolean
  first: boolean
  last: boolean
  busy: boolean
  onToggle: (done: boolean) => void
  onEdit: () => void
  onMove: (direction: -1 | 1) => void
}

/**
 * Uma etapa. Com tarefa, o estado é o da tarefa e o checkbox fica desabilitado: concluir, reatribuir ou recusar
 * acontece na tela de Tarefas. A etapa nunca guarda um estado que discorde da tarefa.
 */
export function StepRow({ step, ctx, editable, first, last, busy, onToggle, onEdit, onMove }: Props) {
  const { t, i18n } = useTranslation('goals')
  const noteId = useId()
  const declined = step.has_task && step.task_status === 'declined'
  const assignee = step.assignee_id ? nameOf(ctx, step.assignee_id) : ''

  return (
    <li className={styles.stepRow} data-done={step.done || undefined}>
      <input
        type="checkbox"
        checked={step.done}
        disabled={step.has_task || !editable || busy}
        aria-label={step.title}
        aria-describedby={step.has_task ? noteId : undefined}
        onChange={(e) => onToggle(e.target.checked)}
      />
      <span className={styles.itemMain}>
        <span className={styles.itemTitle}>{step.title}</span>
        <span className={styles.meta}>
          {step.has_task && (
            <span id={noteId}>{declined ? t('step.declined') : t(step.done ? 'step.doneInTask' : 'step.pendingInTask')}</span>
          )}
          {assignee && !declined && <span>{assignee}</span>}
          {step.due_on && <span>{t('step.dueOn', { date: formatDay(step.due_on, i18n.language) })}</span>}
        </span>
      </span>
      {editable && (
        <span className={styles.actions}>
          <Button variant="quiet" disabled={first || busy} onClick={() => onMove(-1)} aria-label={t('step.moveUp', { title: step.title })}>
            <ArrowUp size={18} aria-hidden="true" />
          </Button>
          <Button variant="quiet" disabled={last || busy} onClick={() => onMove(1)} aria-label={t('step.moveDown', { title: step.title })}>
            <ArrowDown size={18} aria-hidden="true" />
          </Button>
          <Button variant="quiet" onClick={onEdit} aria-label={t('step.edit', { title: step.title })}>
            {t('step.editShort')}
          </Button>
        </span>
      )}
    </li>
  )
}
