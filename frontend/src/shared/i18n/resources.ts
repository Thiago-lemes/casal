import ptCommon from './locales/pt-BR/common.json'
import ptAuth from './locales/pt-BR/auth.json'
import ptToday from './locales/pt-BR/today.json'
import ptHousehold from './locales/pt-BR/household.json'
import ptPush from './locales/pt-BR/push.json'
import ptTasks from './locales/pt-BR/tasks.json'
import ptProfile from './locales/pt-BR/profile.json'
import ptNotes from './locales/pt-BR/notes.json'
import ptRoutine from './locales/pt-BR/routine.json'
import ptReminders from './locales/pt-BR/reminders.json'
import ptShopping from './locales/pt-BR/shopping.json'
import enCommon from './locales/en/common.json'
import enAuth from './locales/en/auth.json'
import enToday from './locales/en/today.json'
import enHousehold from './locales/en/household.json'
import enPush from './locales/en/push.json'
import enTasks from './locales/en/tasks.json'
import enProfile from './locales/en/profile.json'
import enNotes from './locales/en/notes.json'
import enRoutine from './locales/en/routine.json'
import enReminders from './locales/en/reminders.json'
import enShopping from './locales/en/shopping.json'

export const resources = {
  'pt-BR': { common: ptCommon, auth: ptAuth, today: ptToday, household: ptHousehold, push: ptPush, tasks: ptTasks, profile: ptProfile, notes: ptNotes, routine: ptRoutine, reminders: ptReminders, shopping: ptShopping },
  en: { common: enCommon, auth: enAuth, today: enToday, household: enHousehold, push: enPush, tasks: enTasks, profile: enProfile, notes: enNotes, routine: enRoutine, reminders: enReminders, shopping: enShopping },
} as const
