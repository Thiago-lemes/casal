import { useMe } from '@/features/auth/hooks'
import { NotesPage } from '@/features/notes/NotesPage'

export function NotesRoute() {
  const me = useMe().data
  if (!me) return null
  return <NotesPage me={me} />
}
