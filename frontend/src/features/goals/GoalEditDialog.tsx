import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import { todayIn } from '@/shared/lib/dates'
import type { Goal } from './api'
import { buildUpdate, formFromGoal, validate } from './create/form'
import type { FieldErrors, Form } from './create/form'
import { GoalFields } from './create/TypeFields'
import { VisibilityChoice } from './create/VisibilityChoice'
import { useGoalActions } from './hooks'
import type { Me } from './types'
import styles from './goals.module.css'

/**
 * Edita a meta com `version`. Em `goal_conflict` recarrega e avisa o que mudou, sem reenviar sozinho:
 * a pessoa confere a versão nova e salva de novo.
 */
export function GoalEditDialog({ goal, me, onClose }: { goal: Goal; me: Me; onClose: () => void }) {
  const { t } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const { update, reload: refetchGoals } = useGoalActions()
  const [form, setForm] = useState<Form>(() => formFromGoal(goal))
  const [errors, setErrors] = useState<FieldErrors>({})
  const [stale, setStale] = useState(false)
  const today = todayIn(me.timezone)
  const set = (patch: Partial<Form>) => setForm((f) => ({ ...f, ...patch }))

  function submit(e: FormEvent) {
    e.preventDefault()
    const found = validate(form, today, { editing: true, unchangedDue: form.dueOn === (goal.due_on ?? '') })
    setErrors(found)
    if (Object.keys(found).length > 0) return
    setStale(false)
    update.mutate(
      { id: goal.id, body: buildUpdate(goal, form) },
      {
        onSuccess: onClose,
        onError: (err) => {
          // outra pessoa mexeu antes: recarrega a meta (a tela de detalhe passa a ter a versão nova) e pede para a pessoa conferir
          if (err instanceof ApiError && err.is('goal_conflict')) {
            setStale(true)
            void refetchGoals()
          }
        },
      },
    )
  }

  // depois do conflito o `goal` já é a versão nova: refaz o formulário a partir dela, para a pessoa ver o que mudou
  function reload() {
    setForm(formFromGoal(goal))
    setStale(false)
    update.reset()
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()} title={t('edit.title')}>
      <form className={styles.form} onSubmit={submit} noValidate>
        <GoalFields form={form} set={set} errors={errors} editing today={today} />
        {goal.scope === 'personal' && <VisibilityChoice value={form.visibility} onChange={(visibility) => set({ visibility })} />}

        {stale ? (
          <div className={styles.notice} role="alert">
            <p>{t('edit.conflict')}</p>
            <Button onClick={reload}>{t('edit.loadNew')}</Button>
          </div>
        ) : (
          update.error != null && (
            <p className={styles.error} role="alert">
              {errorMessage(tc, update.error)}
            </p>
          )
        )}

        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button type="submit" variant="primary" loading={update.isPending} disabled={stale}>
            {t('edit.save')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
