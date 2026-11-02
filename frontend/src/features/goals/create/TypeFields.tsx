import { useTranslation } from 'react-i18next'
import { TextArea, TextField } from '@/design-system'
import type { FinanceOptions } from '../types'
import { FinancialFields } from './FinancialFields'
import { HabitFields } from './HabitFields'
import type { FieldsProps } from './HabitFields'
import { ProjectFields } from './ProjectFields'
import { SharedFields } from './SharedFields'
import styles from '../goals.module.css'

/** Título, notas, prazo e os campos do tipo. Serve à criação e à edição (na edição, o que não muda some). */
export function GoalFields({ form, set, errors, editing, today, finance }: FieldsProps & { today: string; finance?: FinanceOptions }) {
  const { t } = useTranslation('goals')
  const props = { form, set, errors, editing }
  return (
    <div className={styles.form}>
      <TextField
        label={t('form.title')}
        value={form.title}
        maxLength={120}
        onChange={(e) => set({ title: e.target.value })}
        error={errors.title ? t(`form.errors.${errors.title}`) : undefined}
        autoFocus
        required
      />

      {form.kind === 'habit' && <HabitFields {...props} />}
      {form.kind === 'shared' && <SharedFields {...props} />}
      {form.kind === 'financial' && <FinancialFields {...props} finance={finance} />}
      {form.kind === 'project' && !editing && <ProjectFields {...props} />}

      <TextField
        label={t('form.due')}
        type="date"
        // na edição um prazo antigo continua válido; quem decide é a validação, não o navegador
        min={editing ? undefined : today}
        value={form.dueOn}
        onChange={(e) => set({ dueOn: e.target.value })}
        hint={t('form.dueHint')}
        error={errors.dueOn ? t(`form.errors.${errors.dueOn}`) : undefined}
      />
      <TextArea label={t('form.notes')} value={form.notes} maxLength={1000} onChange={(e) => set({ notes: e.target.value })} />
    </div>
  )
}
