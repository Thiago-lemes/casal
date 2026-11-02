import { useId, useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { todayIn } from '@/shared/lib/dates'
import type { Goal, Step } from '../api'
import { useStepActions } from '../hooks'
import type { Ctx } from '../types'
import styles from '../goals.module.css'

/**
 * Cria ou edita uma etapa. Etapa com tarefa não troca o responsável aqui (`step_has_task`: isso se faz na tela de Tarefas);
 * virar tarefa e desligar a tarefa só existem em projeto do casal.
 */
export function StepEditor({ goal, ctx, step, onClose }: { goal: Goal; ctx: Ctx; step?: Step; onClose: () => void }) {
  const { t } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const actions = useStepActions(goal.id)
  const household = goal.scope === 'household'
  const [title, setTitle] = useState(step?.title ?? '')
  const [dueOn, setDueOn] = useState(step?.due_on ?? '')
  const [assignee, setAssignee] = useState(step?.assignee_id ?? '')
  const [task, setTask] = useState(false)
  const [confirmRemove, setConfirmRemove] = useState(false)
  const [invalid, setInvalid] = useState(false)
  const assigneeId = useId()
  const taskId = useId()

  const mutations = [actions.add, actions.update, actions.remove, actions.toTask, actions.dropTask]
  const error = mutations.map((m) => m.error).find((e) => e != null)
  const busy = mutations.some((m) => m.isPending)
  const editableAssignee = household && !step?.has_task

  function submit(e: FormEvent) {
    e.preventDefault()
    const clean = title.trim()
    if (clean.length < 1 || clean.length > 200) return setInvalid(true)
    setInvalid(false)
    if (!step) {
      actions.add.mutate(
        { title: clean, ...(dueOn ? { due_on: dueOn } : {}), ...(household && assignee ? { assignee_id: assignee } : {}), ...(household && task ? { task: true } : {}) },
        { onSuccess: onClose },
      )
      return
    }
    actions.update.mutate(
      { sid: step.id, body: { title: clean, due_on: dueOn, ...(editableAssignee ? { assignee_id: assignee } : {}) } },
      { onSuccess: onClose },
    )
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()} title={t(step ? 'step.editTitle' : 'step.addTitle')}>
      <form className={styles.form} onSubmit={submit} noValidate>
        <TextField
          label={t('step.title')}
          value={title}
          maxLength={200}
          onChange={(e) => setTitle(e.target.value)}
          error={invalid ? t('form.errors.stepTitle') : undefined}
          autoFocus
          required
        />
        <TextField label={t('step.due')} type="date" min={step ? undefined : todayIn(ctx.me.timezone)} value={dueOn} onChange={(e) => setDueOn(e.target.value)} />

        {household && (
          <div className={styles.field}>
            <label htmlFor={assigneeId} className={styles.label}>
              {t('step.assignee')}
            </label>
            <select id={assigneeId} className={styles.select} value={assignee} disabled={!editableAssignee} onChange={(e) => setAssignee(e.target.value)}>
              <option value="">{t('step.noAssignee')}</option>
              {ctx.people.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
            {!editableAssignee && <p className={styles.hint}>{t('step.assigneeInTask')}</p>}
          </div>
        )}

        {household && !step && (
          <label className={styles.check} htmlFor={taskId}>
            <input id={taskId} type="checkbox" checked={task} onChange={(e) => setTask(e.target.checked)} />
            <span>
              {t('step.asTask')}
              <small>{t('step.asTaskHint')}</small>
            </span>
          </label>
        )}

        {step && household && (
          <div className={styles.actions}>
            {step.has_task ? (
              <Button loading={actions.dropTask.isPending} disabled={busy} onClick={() => actions.dropTask.mutate(step.id, { onSuccess: onClose })}>
                {t('step.dropTask')}
              </Button>
            ) : (
              !step.done && (
                <Button loading={actions.toTask.isPending} disabled={busy} onClick={() => actions.toTask.mutate(step.id, { onSuccess: onClose })}>
                  {t('step.toTask')}
                </Button>
              )
            )}
          </div>
        )}

        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}

        {step && confirmRemove && (
          <div className={styles.notice} role="group" aria-label={t('step.remove')}>
            <p>{t(step.has_task ? 'step.removeConfirmTask' : 'step.removeConfirm')}</p>
            <div className={styles.footer}>
              <Button variant="quiet" onClick={() => setConfirmRemove(false)}>
                {tc('actions.cancel')}
              </Button>
              <Button variant="danger" loading={actions.remove.isPending} onClick={() => actions.remove.mutate(step.id, { onSuccess: onClose })}>
                {t('step.removeYes')}
              </Button>
            </div>
          </div>
        )}

        <div className={styles.footer}>
          {step && !confirmRemove && (
            <Button variant="quiet" onClick={() => setConfirmRemove(true)}>
              {t('step.remove')}
            </Button>
          )}
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={actions.add.isPending || actions.update.isPending}>
            {t('step.save')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
