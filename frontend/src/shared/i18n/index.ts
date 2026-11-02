import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import { resources } from './resources'

export const LANGUAGES = ['pt-BR', 'en'] as const
export type Language = (typeof LANGUAGES)[number]

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'pt-BR',
    supportedLngs: [...LANGUAGES],
    load: 'currentOnly',
    ns: ['common', 'auth', 'today', 'household', 'push', 'tasks', 'profile', 'notes', 'routine'],
    defaultNS: 'common',
    interpolation: { escapeValue: false },
    detection: {
      // "pt", "pt-PT", "en-US" etc. caem em pt-BR e en
      convertDetectedLanguage: (lng: string) => (lng.toLowerCase().startsWith('en') ? 'en' : 'pt-BR'),
      order: ['localStorage', 'navigator'], caches: ['localStorage'], lookupLocalStorage: 'casal.lang',
    },
  })

i18n.on('languageChanged', (lng) => {
  document.documentElement.lang = lng
})
document.documentElement.lang = i18n.language

export default i18n
