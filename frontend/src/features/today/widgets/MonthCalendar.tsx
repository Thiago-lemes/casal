import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { dayKey } from '@/features/reminders/stats'
import { Card } from './Card'
import styles from './MonthCalendar.module.css'

/** Mês com os dias que têm lembrete pendente marcados. */
export function MonthCalendar({ busy, now = new Date() }: { busy: Set<string>; now?: Date }) {
  const { t, i18n } = useTranslation('today')
  const [offset, setOffset] = useState(0)

  const first = new Date(now.getFullYear(), now.getMonth() + offset, 1)
  const daysInMonth = new Date(first.getFullYear(), first.getMonth() + 1, 0).getDate()
  const lead = first.getDay() // domingo = 0

  const monthLabel = new Intl.DateTimeFormat(i18n.language, { month: 'long', year: 'numeric' }).format(first)
  // 7 dias a partir de um domingo qualquer, só para obter as letras no idioma atual
  const weekdays = Array.from({ length: 7 }, (_, i) =>
    new Intl.DateTimeFormat(i18n.language, { weekday: 'narrow' }).format(new Date(2026, 10, 1 + i)),
  )
  const cells: (number | null)[] = [...Array(lead).fill(null), ...Array.from({ length: daysInMonth }, (_, i) => i + 1)]

  return (
    <Card
      title={monthLabel}
      aside={
        <div className={styles.nav}>
          <button type="button" className={styles.step} onClick={() => setOffset((o) => o - 1)} aria-label={t('calendar.prev')}>
            ‹
          </button>
          <button type="button" className={styles.step} onClick={() => setOffset((o) => o + 1)} aria-label={t('calendar.next')}>
            ›
          </button>
        </div>
      }
    >
      <table className={styles.table}>
        <thead>
          <tr>
            {weekdays.map((w, i) => (
              <th key={i} scope="col" className={styles.dow}>
                {w}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: Math.ceil(cells.length / 7) }, (_, row) => (
            <tr key={row}>
              {cells.slice(row * 7, row * 7 + 7).map((day, col) => {
                if (day === null) return <td key={col} />
                const key = dayKey(new Date(first.getFullYear(), first.getMonth(), day))
                const isToday = key === dayKey(now)
                const isBusy = busy.has(key)
                return (
                  <td key={col} className={styles.cell}>
                    <span
                      className={styles.day}
                      data-today={isToday || undefined}
                      data-busy={isBusy || undefined}
                      aria-label={isBusy ? t('calendar.busy', { day }) : undefined}
                    >
                      {day}
                    </span>
                  </td>
                )
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  )
}
