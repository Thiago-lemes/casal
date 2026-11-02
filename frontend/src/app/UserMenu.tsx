import * as Menu from '@radix-ui/react-dropdown-menu'
import { Bell, BellOff, BellRing, CalendarClock, Globe, LogOut, Monitor, Moon, Sun, UserRound } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { Avatar } from '@/design-system'
import { useLogout, useMe } from '@/features/auth/hooks'
import { usePush } from '@/features/push/hooks'
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

/** Avatar da conta; abre tema, avisos, idioma e sair. `side` diz para que lado o menu abre. */
export function UserMenu({ side = 'top' }: { side?: 'top' | 'bottom' }) {
  const { t, i18n } = useTranslation()
  const me = useMe().data
  const [theme, setTheme] = useTheme()
  const logout = useLogout()
  const navigate = useNavigate()
  const push = usePush()
  const { t: tp } = useTranslation('push')
  const current: Language = i18n.language.startsWith('en') ? 'en' : 'pt-BR'

  if (!me) return null

  return (
    <Menu.Root>
      <Menu.Trigger className={styles.trigger} aria-label={t('userMenu.label', { name: me.name })}>
        <Avatar name={me.name} />
        <span className={styles.triggerName}>{me.name}</span>
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content className={styles.content} side={side} align="start" sideOffset={8} collisionPadding={12}>
          <div className={styles.who}>
            <Avatar name={me.name} />
            <div className={styles.whoText}>
              <p className={styles.name}>{me.name}</p>
              <p className={styles.email}>{me.email}</p>
            </div>
          </div>

          <Menu.Separator className={styles.separator} />

          {/* no celular a Semana sai da barra inferior e mora aqui */}
          <Menu.Item className={`${styles.item} ${styles.mobileOnly}`} onSelect={() => navigate('/routine')}>
            <CalendarClock className={styles.icon} aria-hidden="true" />
            {t('nav.routine')}
          </Menu.Item>

          <Menu.Item className={styles.item} onSelect={() => navigate('/profile')}>
            <UserRound className={styles.icon} aria-hidden="true" />
            {t('userMenu.profile')}
          </Menu.Item>

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
            <Menu.Label className={styles.label}>{tp('menu.title')}</Menu.Label>
            {push.status === 'on' || push.status === 'off' ? (
              <>
                <Menu.CheckboxItem
                  className={styles.item}
                  checked={push.status === 'on'}
                  disabled={push.enable.isPending || push.disable.isPending}
                  onSelect={(e) => e.preventDefault()}
                  onCheckedChange={(on) => (on ? push.enable.mutate() : push.disable.mutate())}
                >
                  {push.status === 'on' ? <Bell className={styles.icon} aria-hidden="true" /> : <BellOff className={styles.icon} aria-hidden="true" />}
                  {tp('menu.toggle')}
                  <Menu.ItemIndicator className={styles.check} aria-hidden="true" />
                </Menu.CheckboxItem>
                {push.status === 'on' && (
                  <Menu.Item className={styles.item} disabled={push.test.isPending} onSelect={(e) => { e.preventDefault(); push.test.mutate() }}>
                    <BellRing className={styles.icon} aria-hidden="true" />
                    {push.test.isError ? tp('menu.testFailed') : push.test.isSuccess ? tp('menu.testSent') : tp('menu.test')}
                  </Menu.Item>
                )}
              </>
            ) : (
              push.status && (
                <Menu.Item className={styles.item} disabled>
                  <BellOff className={styles.icon} aria-hidden="true" />
                  {tp(`menu.${push.status}`)}
                </Menu.Item>
              )
            )}
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
