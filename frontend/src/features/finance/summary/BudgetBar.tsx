import { useTranslation } from 'react-i18next'
import { formatBRL } from '../money'
import { budgetTone } from './summary'
import styles from './Summary.module.css'

/** Barra de uma categoria. Com limite, a cor muda em 80% e 100%, e o texto diz o mesmo ("85% do limite"). */
export function BudgetBar({
  name,
  totalCents,
  limitCents,
  usedPercent,
  maxCents,
}: {
  name: string
  totalCents: number
  limitCents?: number
  usedPercent?: number
  /** o maior total entre as categorias, para as barras sem limite terem escala */
  maxCents: number
}) {
  const { t, i18n } = useTranslation('finance')
  const hasLimit = limitCents !== undefined && usedPercent !== undefined
  const tone = hasLimit ? budgetTone(usedPercent) : 'neutral'
  const width = hasLimit ? Math.min(100, usedPercent) : maxCents > 0 ? Math.round((totalCents * 100) / maxCents) : 0

  return (
    <li className={styles.bar}>
      <div className={styles.barHead}>
        <span className={styles.barName}>{name}</span>
        <span className={styles.barValue}>{formatBRL(totalCents, i18n.language)}</span>
      </div>
      <div className={styles.track} aria-hidden="true">
        <div className={styles.fill} data-tone={tone} style={{ width: `${width}%` }} />
      </div>
      {hasLimit && (
        <p className={styles.barNote} data-tone={tone}>
          {t('summary.usedOfLimit', { percent: usedPercent, limit: formatBRL(limitCents, i18n.language) })}
          {tone === 'attention' && ` · ${t('summary.nearLimit')}`}
          {tone === 'alert' && ` · ${t('summary.overLimit')}`}
        </p>
      )}
    </li>
  )
}
