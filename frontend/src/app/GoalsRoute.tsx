import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router-dom'
import { useMe } from '@/features/auth/hooks'
import { categoryName } from '@/features/finance/categories'
import { useBudgets, useCategories } from '@/features/finance/hooks'
import { GoalPage } from '@/features/goals/GoalPage'
import { GoalsPage } from '@/features/goals/GoalsPage'
import type { FinanceOptions } from '@/features/goals/types'
import { useHousehold } from '@/features/household/hooks'
import { useModuleEnabled } from '@/shared/modules/hooks'

/**
 * Metas não importa Finanças: aqui se monta o que a criação de uma meta de economia do orçamento precisa
 * (as categorias que têm limite). Com Finanças desligado, `undefined`.
 */
function useFinanceOptions(): FinanceOptions | undefined {
  const { t } = useTranslation('finance')
  const finance = useModuleEnabled('finance')
  const categories = useCategories(false, finance.on)
  const budgets = useBudgets(finance.on)
  return useMemo(() => {
    if (!finance.on) return undefined
    const limited = new Set((budgets.data ?? []).map((b) => b.category_id))
    return { categories: (categories.data ?? []).filter((c) => limited.has(c.id)).map((c) => ({ id: c.id, name: categoryName(t, c) })) }
  }, [finance.on, categories.data, budgets.data, t])
}

function useCtx() {
  const me = useMe().data
  const household = useHousehold().data
  if (!me || !household) return undefined
  return { me, people: household.members, partner: household.members.find((m) => m.id !== me.id) }
}

/** `/goals`: a lista. */
export function GoalsRoute() {
  const ctx = useCtx()
  const finance = useFinanceOptions()
  if (!ctx) return null
  return <GoalsPage ctx={ctx} finance={finance} />
}

/** `/goals/:id`: o detalhe. */
export function GoalRoute() {
  const { id } = useParams()
  const ctx = useCtx()
  const finance = useFinanceOptions()
  if (!ctx || !id) return null
  return <GoalPage id={id} ctx={ctx} finance={finance} />
}
