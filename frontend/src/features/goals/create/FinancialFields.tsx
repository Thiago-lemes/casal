import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { ChoiceGroup, TextField } from '@/design-system'
import { formatBRL, MAX_CENTS, parseMoney } from '@/shared/lib/money'
import type { FinanceOptions } from '../types'
import type { FieldsProps } from './HabitFields'
import styles from '../goals.module.css'

/**
 * Valor a juntar e a forma de medir. A economia do orçamento é só do casal e precisa de Finanças ligado;
 * a forma de medir não muda depois de criada.
 */
export function FinancialFields({ form, set, errors, editing, finance }: FieldsProps & { finance?: FinanceOptions }) {
  const { t, i18n } = useTranslation('goals')
  const categoryId = useId()
  const cents = parseMoney(form.amount)
  const hint = cents !== null && cents > 0 && cents <= MAX_CENTS ? t('form.interpreted', { amount: formatBRL(cents, i18n.language) }) : undefined
  const household = form.scope === 'household'

  return (
    <>
      <TextField
        label={t('form.amount')}
        inputMode="decimal"
        value={form.amount}
        onChange={(e) => set({ amount: e.target.value })}
        hint={hint}
        error={errors.amount ? t(`form.errors.${errors.amount}`) : undefined}
        required
      />

      {!editing && household && (
        <>
          <ChoiceGroup
            legend={t('form.source.legend')}
            options={[
              { value: 'contributions', label: t('form.source.contributions') },
              { value: 'budget_savings', label: t('form.source.budget') },
            ]}
            value={form.source}
            onChange={(source) => set({ source })}
          />
          <p className={styles.hint}>{t(form.source === 'budget_savings' ? 'form.source.budgetHint' : 'form.source.contributionsHint')}</p>
        </>
      )}

      {!editing && household && form.source === 'budget_savings' && (
        <>
          {!finance && (
            <p className={styles.notice}>
              {t('form.source.financeOff')} <Link to="/finance">{t('form.source.financeLink')}</Link>
            </p>
          )}
          {finance && finance.categories.length === 0 && <p className={styles.notice}>{t('form.source.noLimits')}</p>}
          {finance && finance.categories.length > 0 && (
            <div className={styles.field}>
              <label htmlFor={categoryId} className={styles.label}>
                {t('form.category')}
              </label>
              <select id={categoryId} className={styles.select} value={form.categoryId} onChange={(e) => set({ categoryId: e.target.value })} required>
                <option value="" disabled>
                  {t('form.categoryPick')}
                </option>
                {finance.categories.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
              {errors.category && (
                <p className={styles.error} role="alert">
                  {t(`form.errors.${errors.category}`)}
                </p>
              )}
            </div>
          )}
        </>
      )}
    </>
  )
}
