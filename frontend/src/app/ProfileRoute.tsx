import { useMe, useUpdateProfile } from '@/features/auth/hooks'
import { useHousehold, useLeaveHousehold } from '@/features/household/hooks'
import { ProfilePage } from '@/features/profile/ProfilePage'

/** Compõe o perfil com o que vem de auth e de household (as features não se importam entre si). */
export function ProfileRoute() {
  const me = useMe().data
  const household = useHousehold().data
  const update = useUpdateProfile()
  const leave = useLeaveHousehold()
  if (!me) return null
  return <ProfilePage me={me} update={update} leave={household ? leave : undefined} />
}
