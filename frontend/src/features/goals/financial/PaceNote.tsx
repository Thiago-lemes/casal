import { useTranslation } from 'react-i18next'
import { formatBRL } from '@/shared/lib/money'
import type { Progress } from '../api'
import styles from '../goals.module.css'

/**
 * O ritmo de uma meta com prazo, em tom neutro ("No ritmo" / "Abaixo do ritmo"; sem vermelho de alarme).
 * Sem prazo o servidor não manda nada disto, e a meta batida perde o "por mês".
 */
export function PaceNote({ progress }: { progress: Progress }) {
  const { t, i18n } = useTranslation('goals')
  const perMonth = progress.required_per_month_cents
  if (perMonth === undefined && progress.on_track === undefined) return null
  return (
    <p className={styles.meta}>
      {perMonth !== undefined && perMonth > 0 && <span>{t('financial.perMonth', { amount: formatBRL(perMonth, i18n.language) })}</span>}
      {progress.on_track !== undefined && (
        <span className={styles.badge} data-tone={progress.on_track ? 'ok' : undefined}>
          {t(progress.on_track ? 'financial.onTrack' : 'financial.belowPace')}
        </span>
      )}
    </p>
  )
}
