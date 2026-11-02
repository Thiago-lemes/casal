import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { formatBRL } from '@/shared/lib/money'
import type { Progress } from './api'
import styles from './goals.module.css'

/** O texto do progresso por unidade. Só formata o que o servidor calculou; nunca soma. */
export function progressText(t: TFunction, p: Progress, lang: string): string {
  if (p.unavailable) return t('progress.unavailable')
  if (p.unit === 'cents') return t('progress.cents', { done: formatBRL(p.done, lang), total: formatBRL(p.total, lang) })
  if (p.unit === 'steps') return t('progress.steps', { done: p.done, total: p.total })
  return t('progress.count', { done: p.done, total: p.total })
}

/** `percent` passa de 100 em hábito e meta do casal: a barra limita, o dado fica. */
export const barPercent = (p: Progress) => Math.max(0, Math.min(100, p.percent))

export function GoalProgress({ progress, label }: { progress: Progress; label: string }) {
  const { t, i18n } = useTranslation('goals')
  const text = progressText(t, progress, i18n.language)
  const percent = barPercent(progress)
  return (
    <div className={styles.progress} data-unavailable={progress.unavailable || undefined}>
      <div className={styles.progressHead}>
        <span className={styles.progressText}>{text}</span>
        {!progress.unavailable && <span>{progress.percent}%</span>}
      </div>
      <div
        className={styles.track}
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={progress.unavailable ? undefined : percent}
        aria-valuetext={text}
      >
        <div className={styles.fill} style={{ width: `${progress.unavailable ? 0 : percent}%` }} />
      </div>
    </div>
  )
}
