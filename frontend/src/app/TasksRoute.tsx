import { useMe } from '@/features/auth/hooks'
import { useHousehold } from '@/features/household/hooks'
import { TasksPage } from '@/features/tasks/TasksPage'

/** Compõe a página de tarefas com quem está logado e o casal (as features não se importam entre si). */
export function TasksRoute() {
  const me = useMe().data
  const household = useHousehold().data
  if (!me || !household) return null
  return <TasksPage me={me} people={household.members} />
}
