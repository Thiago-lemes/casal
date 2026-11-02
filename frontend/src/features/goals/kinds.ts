import { ListChecks, PiggyBank, Repeat, Users } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { Kind } from './api'

export const KIND_ICONS: Record<Kind, LucideIcon> = {
  habit: Repeat,
  shared: Users,
  financial: PiggyBank,
  project: ListChecks,
}
