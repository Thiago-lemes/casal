import { useTranslation } from 'react-i18next'
import { ChoiceGroup, EmptyState } from '@/design-system'
import type { FreeWindow } from './api'
import { duration, formatClock } from './dates'
import styles from './FreeView.module.css'

export const MIN_OPTIONS = [30, 60, 90, 120] as const
export type MinOption = (typeof MIN_OPTIONS)[number]

type Props = {
  windows: FreeWindow[]
  timezone: string
  min: MinOption
  onMinChange: (min: MinOption) => void
}

/** Os horários em que os dois estão livres ao mesmo tempo, úteis para cozinhar, passear ou resolver pendências. */
export function FreeView({ windows, timezone, min, onMinChange }: Props) {
  const { t, i18n } = useTranslation('routine')

  return (
    <div className={styles.wrap}>
      <ChoiceGroup
        legend={t('free.min')}
        value={String(min)}
        onChange={(v) => onMinChange(Number(v) as MinOption)}
        options={MIN_OPTIONS.map((m) => ({ value: String(m), label: duration(m) }))}
      />

      {windows.length === 0 ? (
        <EmptyState title={t('free.none.title')} body={t('free.none.body', { min: duration(min) })} />
      ) : (
        <ul className={styles.list}>
          {windows.map((w) => {
            const minutes = Math.round((new Date(w.end_at).getTime() - new Date(w.start_at).getTime()) / 60000)
            return (
              <li key={w.start_at} className={styles.window}>
                <span className={styles.range}>
                  {formatClock(w.start_at, i18n.language, timezone)} – {formatClock(w.end_at, i18n.language, timezone)}
                </span>
                <span className={styles.len}>{duration(minutes)}</span>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
