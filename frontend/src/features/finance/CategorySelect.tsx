import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { Category } from './api'
import { categoryName } from './categories'
import styles from './finance.module.css'

export function CategorySelect({ categories, value, onChange, label }: { categories: Category[]; value: string; onChange: (id: string) => void; label?: string }) {
  const { t } = useTranslation('finance')
  const id = useId()
  return (
    <div className={styles.field}>
      <label htmlFor={id} className={styles.label}>
        {label ?? t('form.category')}
      </label>
      <select id={id} className={styles.select} value={value} onChange={(e) => onChange(e.target.value)}>
        {categories.map((c) => (
          <option key={c.id} value={c.id}>
            {categoryName(t, c)}
          </option>
        ))}
      </select>
    </div>
  )
}
