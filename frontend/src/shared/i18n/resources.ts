import ptCommon from './locales/pt-BR/common.json'
import ptAuth from './locales/pt-BR/auth.json'
import ptToday from './locales/pt-BR/today.json'
import ptHousehold from './locales/pt-BR/household.json'
import enCommon from './locales/en/common.json'
import enAuth from './locales/en/auth.json'
import enToday from './locales/en/today.json'
import enHousehold from './locales/en/household.json'

export const resources = {
  'pt-BR': { common: ptCommon, auth: ptAuth, today: ptToday, household: ptHousehold },
  en: { common: enCommon, auth: enAuth, today: enToday, household: enHousehold },
} as const
