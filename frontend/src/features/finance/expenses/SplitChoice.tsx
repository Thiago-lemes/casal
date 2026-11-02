import { useTranslation } from 'react-i18next'
import { ChoiceGroup } from '@/design-system'
import type { Split } from '../api'
import type { SplitMode } from '../split'
import type { Ctx } from '../types'
import { SplitEditor } from './SplitEditor'

/** Quem paga e como se divide, para as telas que criam gastos sem passar pelo `ExpenseComposer` (fixos). */
export function SplitChoice({
  ctx,
  paidBy,
  onPaidBy,
  mode,
  onMode,
  custom,
  onCustom,
}: {
  ctx: Ctx
  paidBy: string
  onPaidBy: (id: string) => void
  mode: SplitMode
  onMode: (m: SplitMode) => void
  custom: Split
  onCustom: (s: Split) => void
}) {
  const { t } = useTranslation('finance')
  if (!ctx.partner) return null
  return (
    <>
      <ChoiceGroup
        legend={t('form.paidBy')}
        options={ctx.people.map((p) => ({ value: p.id, label: p.id === ctx.me.id ? t('form.me') : p.name }))}
        value={paidBy}
        onChange={onPaidBy}
      />
      <ChoiceGroup
        legend={t('split.legend')}
        options={[
          { value: 'default', label: t('split.default') },
          { value: 'equal', label: t('split.equal') },
          { value: 'mine', label: t('split.mine') },
          { value: 'theirs', label: t('split.theirs', { name: ctx.partner.name }) },
          { value: 'custom', label: t('split.custom') },
        ]}
        value={mode}
        onChange={onMode}
      />
      {mode === 'custom' && <SplitEditor people={ctx.people} value={custom} onChange={onCustom} />}
    </>
  )
}
