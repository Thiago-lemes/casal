import { Navigate, Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { LoadingScreen } from '@/design-system'
import { useMe } from '@/features/auth/hooks'
import { useHousehold } from '@/features/household/hooks'
import { SetupPage } from '@/features/household/SetupPage'

function Loading() {
  const { t } = useTranslation()
  return <LoadingScreen label={t('loading')} />
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
