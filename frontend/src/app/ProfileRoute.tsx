import { useEffect } from 'react'
import { useMe, useUpdateProfile } from '@/features/auth/hooks'
import { useHousehold, useLeaveHousehold } from '@/features/household/hooks'
import { exportUrl } from '@/features/finance/api'
import { goalsExportUrl } from '@/features/goals/api'
import { writeSaved } from '@/features/onboarding/progress'
import { ProfilePage } from '@/features/profile/ProfilePage'
import { useModuleEnabled } from '@/shared/modules/hooks'

/** Compõe o perfil com o que vem de auth e de household (as features não se importam entre si). */
export function ProfileRoute() {
  const me = useMe().data
  const household = useHousehold().data
  const update = useUpdateProfile()
  const leave = useLeaveHousehold()
  const finance = useModuleEnabled('finance', Boolean(household))
  const goals = useModuleEnabled('goals', Boolean(household))
  // abrir o perfil conta como conferir a cidade (primeiros passos): a cidade pode já estar certa
  const meId = me?.id
  useEffect(() => {
    if (meId) writeSaved(meId, { cityChecked: true })
  }, [meId])
  if (!me) return null
  return <ProfilePage me={me} update={update} leave={household ? leave : undefined} exportHref={finance.on ? exportUrl('csv') : undefined} goalsExportHref={goals.on ? goalsExportUrl('csv') : undefined} />
}
