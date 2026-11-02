import { useMe } from '@/features/auth/hooks'
import { useHousehold } from '@/features/household/hooks'
import { RoutinePage } from '@/features/routine/RoutinePage'

export function RoutineRoute() {
  const me = useMe().data
  const household = useHousehold().data
  if (!me || !household) return null
  return <RoutinePage me={me} people={household.members} />
}
