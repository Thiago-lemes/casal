import { useMe } from '@/features/auth/hooks'
import { useHousehold } from '@/features/household/hooks'
import { RemindersPage } from '@/features/reminders/RemindersPage'

export function RemindersRoute() {
  const me = useMe().data
  const household = useHousehold().data
  if (!me || !household) return null
  return <RemindersPage me={me} partner={household.members.find((m) => m.id !== me.id)} />
}
