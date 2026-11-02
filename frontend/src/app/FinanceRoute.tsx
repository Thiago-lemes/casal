import { useMe } from '@/features/auth/hooks'
import { FinancePage } from '@/features/finance/FinancePage'
import { useHousehold } from '@/features/household/hooks'

/** Compõe Finanças com quem está logado e o casal (as features não se importam entre si). */
export function FinanceRoute() {
  const me = useMe().data
  const household = useHousehold().data
  if (!me || !household) return null
  return <FinancePage me={me} people={household.members} />
}
