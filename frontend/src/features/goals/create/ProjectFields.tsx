import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, TextField } from '@/design-system'
import { MAX_STEPS } from './form'
import type { FieldsProps } from './HabitFields'
import styles from '../goals.module.css'

/** Etapas na criação: um título por linha. As etapas com tarefa só existem em projeto do casal. */
export function ProjectFields({ form, set, errors }: FieldsProps) {
  const { t } = useTranslation('goals')
  const taskId = useId()

  const change = (i: number, value: string) => set({ steps: form.steps.map((s, j) => (j === i ? value : s)) })

  return (
    <fieldset className={styles.field}>
      <legend className={styles.legend}>{t('form.steps.legend')}</legend>
      <div className={styles.stepInputs}>
        {form.steps.map((s, i) => (
          <div key={i} className={styles.inline}>
            <TextField label={t('form.steps.item', { n: i + 1 })} value={s} maxLength={200} onChange={(e) => change(i, e.target.value)} />
            {form.steps.length > 1 && (
              <Button variant="quiet" onClick={() => set({ steps: form.steps.filter((_, j) => j !== i) })} aria-label={t('form.steps.remove', { n: i + 1 })}>
                {t('form.steps.removeShort')}
              </Button>
            )}
          </div>
        ))}
      </div>
      {errors.steps && (
        <p className={styles.error} role="alert">
          {t(`form.errors.${errors.steps}`)}
        </p>
      )}
      <div>
        <Button variant="quiet" disabled={form.steps.length >= MAX_STEPS} onClick={() => set({ steps: [...form.steps, ''] })}>
          {t('form.steps.add')}
        </Button>
      </div>
      {form.scope === 'household' && (
        <label className={styles.check} htmlFor={taskId}>
          <input id={taskId} type="checkbox" checked={form.stepTasks} onChange={(e) => set({ stepTasks: e.target.checked })} />
          <span>
            {t('form.steps.tasks')}
            <small>{t('form.steps.tasksHint')}</small>
          </span>
        </label>
      )}
    </fieldset>
  )
}
