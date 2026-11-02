import { useTranslation } from 'react-i18next'
import { ChoiceGroup, TextField } from '@/design-system'
import { ReminderField } from '../habit/ReminderField'
import { WeekdayPicker } from '../habit/WeekdayPicker'
import type { FieldErrors, Form } from './form'
import styles from '../goals.module.css'

export type FieldsProps = { form: Form; set: (patch: Partial<Form>) => void; errors: FieldErrors; editing?: boolean }

/** Quantas vezes por semana ou mês; o período não muda depois de criada. */
export function TargetFields({ form, set, errors, editing }: FieldsProps) {
  const { t } = useTranslation('goals')
  return (
    <>
      {editing ? (
        <p className={styles.hint}>{t(form.period === 'week' ? 'form.periodFixedWeek' : 'form.periodFixedMonth')}</p>
      ) : (
        <ChoiceGroup
          legend={t('form.period')}
          options={[
            { value: 'week', label: t('form.periodWeek') },
            { value: 'month', label: t('form.periodMonth') },
          ]}
          value={form.period}
          onChange={(period) => set({ period })}
        />
      )}
      <TextField
        label={t(form.period === 'week' ? 'form.targetWeek' : 'form.targetMonth')}
        inputMode="numeric"
        value={form.target}
        onChange={(e) => set({ target: e.target.value })}
        error={errors.target ? t(`form.errors.${errors.target}`) : undefined}
        required
      />
    </>
  )
}

export function HabitFields(props: FieldsProps) {
  const { form, set } = props
  return (
    <>
      <TargetFields {...props} />
      <WeekdayPicker days={form.days} onChange={(days) => set({ days })} />
      <ReminderField on={form.remindOn} time={form.remindTime} onChange={set} />
    </>
  )
}
