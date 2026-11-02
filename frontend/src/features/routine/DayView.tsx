import { useTranslation } from 'react-i18next'
import { CATEGORY_ICONS } from './categories'
import { clock, formatClock } from './dates'
import { placeDay, visibleRange } from './layout'
import type { Occurrence } from './api'
import type { Me, Person } from './types'
import { toneOf } from './types'
import styles from './DayView.module.css'

type Props = {
  day: string
  occurrences: Occurrence[]
  me: Me
  partner?: Person
  onSelect: (occ: Occurrence) => void
}

const PX_PER_HOUR = 56

/** O dia em colunas: a sua e a da outra pessoa, lado a lado, na mesma escala de horas. */
export function DayView({ day, occurrences, me, partner, onSelect }: Props) {
  const { t, i18n } = useTranslation('routine')

  const columns = [
    { person: me as Person, tone: 'me' as const },
    ...(partner ? [{ person: partner, tone: 'partner' as const }] : []),
  ]
  const byColumn = columns.map((c) => placeDay(occurrences.filter((o) => toneOf(o.owner_id, me.id) === c.tone), day, me.timezone))
  const range = visibleRange(byColumn.flat())
  const hours = Array.from({ length: (range.to - range.from) / 60 + 1 }, (_, i) => range.from + i * 60)
  const height = ((range.to - range.from) / 60) * PX_PER_HOUR

  const empty = byColumn.every((c) => c.length === 0)

  return (
    <div className={styles.wrap}>
      <div className={styles.head} style={{ gridTemplateColumns: `3rem repeat(${columns.length}, 1fr)` }}>
        <span />
        {columns.map((c) => (
          <p key={c.tone} className={styles.person} data-tone={c.tone}>
            {c.tone === 'me' ? t('day.you') : c.person.name}
          </p>
        ))}
      </div>

      <div className={styles.grid} style={{ gridTemplateColumns: `3rem repeat(${columns.length}, 1fr)`, height }}>
        <div className={styles.axis} aria-hidden="true">
          {hours.map((m) => (
            <span key={m} style={{ top: ((m - range.from) / 60) * PX_PER_HOUR }}>
              {clock(m % 1440 === 0 && m > 0 ? 1440 : m).replace(':00', 'h')}
            </span>
          ))}
        </div>

        {columns.map((c, ci) => (
          <ol key={c.tone} className={styles.column} data-tone={c.tone}>
            {byColumn[ci].map((p) => {
              const Icon = CATEGORY_ICONS[p.occ.category]
              const top = ((p.from - range.from) / 60) * PX_PER_HOUR
              const h = Math.max(((p.to - p.from) / 60) * PX_PER_HOUR, 28)
              const label = p.occ.busy ? t('busy') : p.occ.title
              return (
                <li
                  key={`${p.occ.block_id}-${p.from}`}
                  className={styles.slot}
                  style={{ top, height: h, left: `${(p.lane / p.lanes) * 100}%`, width: `${100 / p.lanes}%` }}
                >
                  <button
                    type="button"
                    className={styles.occ}
                    data-tone={c.tone}
                    data-busy={p.occ.busy || undefined}
                    data-exception={p.occ.exception || undefined}
                    onClick={() => onSelect(p.occ)}
                  >
                    <span className={styles.what}>
                      {!p.occ.busy && <Icon size={14} aria-hidden="true" />}
                      {label}
                    </span>
                    <span className={styles.when}>
                      {formatClock(p.occ.start_at, i18n.language, me.timezone)} – {formatClock(p.occ.end_at, i18n.language, me.timezone)}
                      {p.occ.location && ` · ${p.occ.location}`}
                      {p.occ.exception && ` · ${t('exception.badge')}`}
                    </span>
                  </button>
                </li>
              )
            })}
          </ol>
        ))}
      </div>

      {empty && <p className={styles.empty}>{t('day.empty')}</p>}
    </div>
  )
}
