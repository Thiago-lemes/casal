import { useTranslation } from 'react-i18next'
import { ChoiceGroup } from '@/design-system'
import type { Choice } from './permissions'
import type { Person } from './types'

/** Eu, o parceiro ou "do casal". Sem parceiro, só Eu e Do casal. */
export function AssigneeSelect({
  value,
  onChange,
  partner,
  legend,
}: {
  value: Choice
  onChange: (choice: Choice) => void
  partner?: Person
  legend?: string
}) {
  const { t } = useTranslation('tasks')
  const options: { value: Choice; label: string }[] = [
    { value: 'me', label: t('assignee.me') },
    ...(partner ? [{ value: 'partner' as const, label: partner.name }] : []),
    { value: 'shared', label: t('assignee.shared') },
  ]
  return <ChoiceGroup legend={legend ?? t('assignee.legend')} options={options} value={value} onChange={onChange} />
}
