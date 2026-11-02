import { useTranslation } from 'react-i18next'
import { ChoiceGroup, TextField } from '@/design-system'
import { daysBetween } from '../dates'
import { formatBRL, installmentPreview } from '../money'
import styles from '../finance.module.css'

export type InstallmentValue = { on: boolean; count: string; firstOn: string; mode: 'total' | 'each' }

export const MIN_INSTALLMENTS = 2
export const MAX_INSTALLMENTS = 48

export function installmentCount(v: InstallmentValue): number | null {
  const n = Number(v.count)
  return Number.isInteger(n) && n >= MIN_INSTALLMENTS && n <= MAX_INSTALLMENTS ? n : null
}

/** "Foi parcelado": número de parcelas, data da primeira e se o valor digitado é o total ou o da parcela. */
export function InstallmentFields({
  value,
  onChange,
  today,
  cents,
}: {
  value: InstallmentValue
  onChange: (v: InstallmentValue) => void
  today: string
  /** o valor digitado no campo de valor, já em centavos (ou null) */
  cents: number | null
}) {
  const { t, i18n } = useTranslation('finance')
  const count = installmentCount(value)
  const past = value.firstOn !== '' && daysBetween(today, value.firstOn) < 0

  // só uma dica: o valor que vale é o da resposta do servidor
  let preview: string | undefined
  if (count && cents) {
    if (value.mode === 'each') {
      preview = t('installments.previewEach', { count, each: formatBRL(cents, i18n.language), total: formatBRL(cents * count, i18n.language) })
    } else {
      const { each, exact } = installmentPreview(cents, count)
      preview = t(exact ? 'installments.previewExact' : 'installments.previewApprox', {
        count,
        each: formatBRL(each, i18n.language),
        total: formatBRL(cents, i18n.language),
      })
    }
  }

  return (
    <div className={styles.form}>
      <label className={styles.check}>
        <input type="checkbox" checked={value.on} onChange={(e) => onChange({ ...value, on: e.target.checked })} />
        <span>{t('installments.toggle')}</span>
      </label>

      {value.on && (
        <>
          <div className={styles.row2}>
            <TextField
              label={t('installments.count')}
              type="number"
              inputMode="numeric"
              min={MIN_INSTALLMENTS}
              max={MAX_INSTALLMENTS}
              value={value.count}
              onChange={(e) => onChange({ ...value, count: e.target.value })}
              error={value.count !== '' && !count ? t('installments.countInvalid', { min: MIN_INSTALLMENTS, max: MAX_INSTALLMENTS }) : undefined}
            />
            <TextField
              label={t('installments.firstOn')}
              type="date"
              value={value.firstOn}
              onChange={(e) => onChange({ ...value, firstOn: e.target.value })}
            />
          </div>
          <ChoiceGroup
            legend={t('installments.modeLegend')}
            options={[
              { value: 'total', label: t('installments.modeTotal') },
              { value: 'each', label: t('installments.modeEach') },
            ]}
            value={value.mode}
            onChange={(mode) => onChange({ ...value, mode })}
          />
          {preview && (
            <p className={styles.hint} aria-live="polite">
              {preview}
            </p>
          )}
          {past && <p className={styles.hint}>{t('installments.past')}</p>}
        </>
      )}
    </div>
  )
}
