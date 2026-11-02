import { useTranslation } from 'react-i18next'
import { formatDay } from '@/shared/lib/dates'
import { ALL_DAYS } from '../weekdays'
import styles from '../goals.module.css'

/** Dias combinados. Nenhum marcado quer dizer "qualquer dia" (a máscara 0 do servidor). */
export function WeekdayPicker({ days, onChange }: { days: number[]; onChange: (days: number[]) => void }) {
  const { t, i18n } = useTranslation('goals')

  function toggle(d: number) {
    onChange(days.includes(d) ? days.filter((x) => x !== d) : [...days, d].sort((a, b) => a - b))
  }

  return (
    <fieldset className={styles.field}>
      <legend className={styles.legend}>{t('form.days')}</legend>
      <div className={styles.days}>
        {ALL_DAYS.map((d) => (
          <label key={d} className={styles.dayPick}>
            <input type="checkbox" checked={days.includes(d)} onChange={() => toggle(d)} />
            {/* 2 a 8 de nov/2026 é segunda a domingo */}
            <span>{formatDay(`2026-11-0${d + 1}`, i18n.language, { weekday: 'short' })}</span>
          </label>
        ))}
      </div>
      <p className={styles.hint}>{t('form.daysHint')}</p>
    </fieldset>
  )
}
