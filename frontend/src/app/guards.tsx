import { Navigate, Outlet } from 'react-router-dom'
import { Skeleton } from '@/design-system'
import { useMe } from '@/features/auth/hooks'
import { useHousehold } from '@/features/household/hooks'
import { SetupPage } from '@/features/household/SetupPage'

function Loading() {
  return (
    <div style={{ padding: 'var(--space-5)' }}>
      <Skeleton height="6rem" />
    </div>
  )
}

/** Só passa quem tem sessão; sem sessão, vai para o login. */
export function RequireAuth() {
  const me = useMe()
  if (me.isPending) return <Loading />
  if (!me.data) return <Navigate to="/login" replace />
  return <Outlet />
}

/** Só passa quem já está em um casal; os outros montam ou entram em um. */
export function RequireHousehold() {
  const household = useHousehold()
  if (household.isPending) return <Loading />
  if (!household.data) return <SetupPage />
  return <Outlet />
}
