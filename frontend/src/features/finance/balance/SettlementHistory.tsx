import { useTranslation } from 'react-i18next'
import { Card } from '@/design-system'
import { formatDay } from '../dates'
import { useSettlements } from '../hooks'
import { formatBRL } from '../money'
import { nameOf } from '../types'
import type { Ctx } from '../types'
import styles from '../finance.module.css'

/** Os acertos já registrados. `kind = leave` é o gerado quando alguém saiu do casal com saldo (só registro). */
export function SettlementHistory({ ctx }: { ctx: Ctx }) {
  const { t, i18n } = useTranslation('finance')
  const list = useSettlements()
  const items = list.data ?? []
  if (items.length === 0) return null
  const who = (id: string) => (id === ctx.me.id ? t('form.me') : nameOf(ctx, id))

  return (
    <Card title={t('history.title')}>
      <ul className={styles.list}>
        {items.map((s) => (
          <li key={s.id} className={styles.item}>
            <span className={styles.itemMain}>
              <span className={styles.itemTitle}>{t('settle.direction', { from: who(s.from_user), to: who(s.to_user) })}</span>
              <span className={styles.itemMeta}>
                <span>{formatDay(s.through_on, i18n.language, { day: 'numeric', month: 'short', year: 'numeric' })}</span>
                {s.kind === 'leave' && <span className={styles.badge}>{t('history.leave')}</span>}
              </span>
            </span>
            <span className={styles.money}>{formatBRL(s.amount_cents, i18n.language)}</span>
          </li>
        ))}
      </ul>
    </Card>
  )
}
