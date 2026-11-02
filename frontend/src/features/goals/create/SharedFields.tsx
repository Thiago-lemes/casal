import { useTranslation } from 'react-i18next'
import { ChoiceGroup } from '@/design-system'
import { TargetFields } from './HabitFields'
import type { FieldsProps } from './HabitFields'

export function SharedFields(props: FieldsProps) {
  const { t } = useTranslation('goals')
  const { form, set } = props
  return (
    <>
      <TargetFields {...props} />
      <ChoiceGroup
        legend={t('form.requires.legend')}
        options={[
          { value: 'any', label: t('form.requires.any') },
          { value: 'both', label: t('form.requires.both') },
        ]}
        value={form.requiresBoth ? 'both' : 'any'}
        onChange={(v) => set({ requiresBoth: v === 'both' })}
      />
    </>
  )
}
