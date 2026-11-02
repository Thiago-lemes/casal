import { useTranslation } from 'react-i18next'
import { CATEGORY_ICONS } from './categories'
import { formatClock, formatDay, weekDays } from './dates'
import { minutesInDay } from './layout'
import type { Occurrence } from './api'
import type { Me } from './types'
import { toneOf } from './types'
import styles from './WeekView.module.css'

type Props = {
  /** um dia qualquer da semana a mostrar */
  day: string
  today: string
  occurrences: Occurrence[]
  me: Me
  onSelect: (occ: Occurrence, day: string) => void
  onOpenDay: (day: string) => void
}

/** A semana de segunda a domingo; no celular vira uma lista de dias, no desktop uma grade de sete colunas. */
export function WeekView({ day, today, occurrences, me, onSelect, onOpenDay }: Props) {
  const { t, i18n } = useTranslation('routine')

  return (
    <ol className={styles.week}>
      {weekDays(day).map((d) => {
        const items = occurrences
          .map((o) => ({ o, span: minutesInDay(o, d, me.timezone) }))
          .filter((x): x is { o: Occurrence; span: { from: number; to: number } } => x.span !== null)
          .sort((a, b) => a.span.from - b.span.from)

        return (
          <li key={d} className={styles.day} data-today={d === today || undefined}>
            <button type="button" className={styles.dayHead} onClick={() => onOpenDay(d)} aria-label={t('week.openDay', { day: formatDay(d, i18n.language) })}>
              <span className={styles.dow}>{formatDay(d, i18n.language, { weekday: 'short' })}</span>
              <span className={styles.num}>{formatDay(d, i18n.language, { day: 'numeric' })}</span>
            </button>

            {items.length === 0 ? (
              <p className={styles.none}>{t('week.free')}</p>
            ) : (
              <ul className={styles.list}>
                {items.map(({ o }) => {
                  const Icon = CATEGORY_ICONS[o.category]
                  return (
                    <li key={`${o.block_id}-${o.start_at}`}>
                      <button
                        type="button"
                        className={styles.item}
                        data-tone={toneOf(o.owner_id, me.id)}
                        data-busy={o.busy || undefined}
                        onClick={() => onSelect(o, d)}
                      >
                        <span className={styles.time}>{formatClock(o.start_at, i18n.language, me.timezone)}</span>
                        <span className={styles.what}>
                          {!o.busy && <Icon size={14} aria-hidden="true" />}
                          {o.busy ? t('busy') : o.title}
                        </span>
                        {!o.busy && o.owner_id !== me.id && <span className={styles.who}>{o.owner_name}</span>}
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
          </li>
        )
      })}
    </ol>
  )
}
