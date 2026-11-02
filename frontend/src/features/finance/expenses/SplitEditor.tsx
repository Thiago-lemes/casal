import { useTranslation } from 'react-i18next'
import { splitLabel } from '@/shared/lib/money'
import { setShare, WHOLE } from '../split'
import type { Split } from '../api'
import type { Person } from '../types'
import styles from '../finance.module.css'

/**
 * Um controle de porcentagem entre as duas pessoas; a parte do outro é sempre o que falta para 100%.
 * Trabalha em pontos-base (1% = 100) e nunca deixa a soma sair de 10000.
 */
export function SplitEditor({ people, value, onChange }: { people: Person[]; value: Split; onChange: (s: Split) => void }) {
  const { t, i18n } = useTranslation('finance')
  const [first, second] = people
  const mine = value[first.id] ?? 0

  return (
    <div className={styles.field}>
      <label htmlFor="split-range" className={styles.label}>
        {t('split.editor')}
      </label>
      <input
        id="split-range"
        type="range"
        min={0}
        max={WHOLE}
        step={500}
        value={mine}
        aria-valuetext={`${first.name} ${splitLabel(mine, i18n.language)}`}
        onChange={(e) => onChange(setShare(people, first.id, Number(e.target.value)))}
        style={{ accentColor: 'var(--color-accent)' }}
      />
      <div className={styles.row2}>
        <span>
          {first.name}: <strong>{splitLabel(mine, i18n.language)}</strong>
        </span>
        {second && (
          <span style={{ textAlign: 'right' }}>
            {second.name}: <strong>{splitLabel(value[second.id] ?? WHOLE - mine, i18n.language)}</strong>
          </span>
        )}
      </div>
    </div>
  )
}
