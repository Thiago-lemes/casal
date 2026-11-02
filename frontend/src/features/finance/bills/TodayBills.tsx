import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Card } from '@/design-system'
import type { Bill } from '../api'
import { todayIn } from '@/shared/lib/dates'
import { useBillsDueToday, useModuleGuard } from '../hooks'
import type { Ctx } from '../types'
import { BillRow } from './BillRow'
import { PayDialog } from './PayDialog'
import styles from '../finance.module.css'

/** "Contas de hoje": as abertas que vencem hoje ou já venceram. A tela Hoje só o monta com o módulo ligado. */
export function TodayBills({ ctx }: { ctx: Ctx }) {
  const { t } = useTranslation('finance')
  const list = useBillsDueToday()
  const [paying, setPaying] = useState<Bill>()
  useModuleGuard(list.error)

  const bills = list.data ?? []
  const today = todayIn(ctx.me.timezone)
  const empty = list.isPending || list.isError || bills.length === 0

  // o diálogo vive fora do cartão: ao pagar a última conta a lista esvazia, e ele não pode desmontar no meio
  return (
    <>
      {!empty && (
        <Card title={t('bills.todayTitle')} aside={<Link to="/finance">{t('bills.seeAll')}</Link>}>
          <ul className={styles.list}>
            {bills.map((b) => (
              <BillRow key={b.id} bill={b} ctx={ctx} today={today} onPay={setPaying} />
            ))}
          </ul>
        </Card>
      )}
      {paying && <PayDialog ctx={ctx} bill={paying} onClose={() => setPaying(undefined)} />}
    </>
  )
}
