import { Briefcase, CalendarCheck, Circle, Dumbbell, GraduationCap, House, TrainFront } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { Category } from './api'

export const CATEGORY_ICONS: Record<Category, LucideIcon> = {
  work: Briefcase,
  study: GraduationCap,
  workout: Dumbbell,
  commute: TrainFront,
  appointment: CalendarCheck,
  home: House,
  other: Circle,
}
