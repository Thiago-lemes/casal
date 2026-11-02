import * as Menu from '@radix-ui/react-dropdown-menu'
import { Globe, LogOut, Monitor, Moon, Sun } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Avatar } from '@/design-system'
import { useLogout, useMe } from '@/features/auth/hooks'
import { LANGUAGES } from '@/shared/i18n'
import type { Language } from '@/shared/i18n'
import { useTheme } from '@/shared/theme'
import type { ThemeChoice } from '@/shared/theme'
import styles from './UserMenu.module.css'

const themes: { value: ThemeChoice; key: string; icon: LucideIcon }[] = [
  { value: 'system', key: 'themeSystem', icon: Monitor },
  { value: 'light', key: 'themeLight', icon: Sun },
  { value: 'dark', key: 'themeDark', icon: Moon },
]
const languageNames: Record<Language, string> = { 'pt-BR': 'Português', en: 'English' }

/** Avatar no canto superior esquerdo; abre a conta: tema, idioma e sair. */
export function UserMenu() {
  const { t, i18n } = useTranslation()
  const me = useMe().data
  const [theme, setTheme] = useTheme()
  const logout = useLogout()
  const current: Language = i18n.language.startsWith('en') ? 'en' : 'pt-BR'

  if (!me) return null

  return (
    <Menu.Root>
      <Menu.Trigger className={styles.trigger} aria-label={t('userMenu.label', { name: me.name })}>
        <Avatar name={me.name} />
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content className={styles.content} align="start" sideOffset={8} collisionPadding={12}>
          <div className={styles.who}>
            <Avatar name={me.name} />
            <div className={styles.whoText}>
              <p className={styles.name}>{me.name}</p>
              <p className={styles.email}>{me.email}</p>
            </div>
          </div>

          <Menu.Separator className={styles.separator} />

          <Menu.Group>
            <Menu.Label className={styles.label}>{t('settings.theme')}</Menu.Label>
            <Menu.RadioGroup value={theme} onValueChange={(v) => setTheme(v as ThemeChoice)}>
              {themes.map(({ value, key, icon: Icon }) => (
                <Menu.RadioItem key={value} value={value} className={styles.item}>
                  <Icon className={styles.icon} aria-hidden="true" />
                  {t(`settings.${key}`)}
                  <Menu.ItemIndicator className={styles.check} aria-hidden="true" />
                </Menu.RadioItem>
              ))}
            </Menu.RadioGroup>
          </Menu.Group>

          <Menu.Separator className={styles.separator} />

          <Menu.Group>
            <Menu.Label className={styles.label}>{t('settings.language')}</Menu.Label>
            <Menu.RadioGroup value={current} onValueChange={(v) => void i18n.changeLanguage(v)}>
              {LANGUAGES.map((lng) => (
                <Menu.RadioItem key={lng} value={lng} className={styles.item}>
                  <Globe className={styles.icon} aria-hidden="true" />
                  <span lang={lng}>{languageNames[lng]}</span>
                  <Menu.ItemIndicator className={styles.check} aria-hidden="true" />
                </Menu.RadioItem>
              ))}
            </Menu.RadioGroup>
          </Menu.Group>

          <Menu.Separator className={styles.separator} />

          <Menu.Item className={styles.item} onSelect={() => logout.mutate()}>
            <LogOut className={styles.icon} aria-hidden="true" />
            {t('settings.signOut')}
          </Menu.Item>
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  )
}
