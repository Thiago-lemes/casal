import { useMe, useUpdateProfile } from '@/features/auth/hooks'
import { useHousehold, useLeaveHousehold } from '@/features/household/hooks'
import { exportUrl } from '@/features/finance/api'
import { ProfilePage } from '@/features/profile/ProfilePage'
import { useModuleEnabled } from '@/shared/modules/hooks'

/** Compõe o perfil com o que vem de auth e de household (as features não se importam entre si). */
export function ProfileRoute() {
  const me = useMe().data
  const household = useHousehold().data
  const update = useUpdateProfile()
  const leave = useLeaveHousehold()
  const finance = useModuleEnabled('finance', Boolean(household))
  if (!me) return null
  return <ProfilePage me={me} update={update} leave={household ? leave : undefined} exportHref={finance.on ? exportUrl('csv') : undefined} />
}
