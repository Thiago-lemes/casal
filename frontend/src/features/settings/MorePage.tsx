import { useTranslation } from 'react-i18next'
import { Button } from '@/design-system'
import { useLogout } from '@/features/auth/hooks'
import { LANGUAGES } from '@/shared/i18n'
import type { Language } from '@/shared/i18n'
import { useTheme } from '@/shared/theme'
import type { ThemeChoice } from '@/shared/theme'
import styles from './MorePage.module.css'

const themes: ThemeChoice[] = ['system', 'light', 'dark']
const labels: Record<Language, string> = { 'pt-BR': 'Português', en: 'English' }

/** "Mais": preferências do aparelho (tema, idioma) e sair da conta. */
export function MorePage() {
  const { t, i18n } = useTranslation()
  const [theme, setTheme] = useTheme()
  const logout = useLogout()
  const current: Language = i18n.language.startsWith('en') ? 'en' : 'pt-BR'

  return (
    <div className={styles.page}>
      <h1 className={styles.title}>{t('settings.title')}</h1>

      <fieldset className={styles.group}>
        <legend className={styles.legend}>{t('settings.theme')}</legend>
        <div className={styles.options}>
          {themes.map((choice) => (
            <label key={choice} className={styles.option}>
              <input
                type="radio"
                name="theme"
                checked={theme === choice}
                onChange={() => setTheme(choice)}
              />
              <span>{t(`settings.theme${choice[0].toUpperCase()}${choice.slice(1)}`)}</span>
            </label>
          ))}
        </div>
      </fieldset>

      <fieldset className={styles.group}>
        <legend className={styles.legend}>{t('settings.language')}</legend>
        <div className={styles.options}>
          {LANGUAGES.map((lng) => (
            <label key={lng} className={styles.option}>
              <input
                type="radio"
                name="language"
                checked={current === lng}
                onChange={() => void i18n.changeLanguage(lng)}
              />
              <span lang={lng}>{labels[lng]}</span>
            </label>
          ))}
        </div>
      </fieldset>

      <Button loading={logout.isPending} onClick={() => logout.mutate()}>
        {t('settings.signOut')}
      </Button>
    </div>
  )
}
