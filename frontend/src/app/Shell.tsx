import { CalendarClock, House, LayoutDashboard } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { NavLink, Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { APP_NAME } from '@/shared/config'
import styles from './Shell.module.css'
import { UserMenu } from './UserMenu'

type Item = { to: string; key: string; icon: LucideIcon; end?: boolean; soon?: boolean }

const items: Item[] = [
  { to: '/', key: 'today', icon: LayoutDashboard, end: true },
  { to: '/routine', key: 'routine', icon: CalendarClock, soon: true },
  { to: '/home', key: 'home', icon: House },
]

/** Moldura do app: barra inferior no celular, trilho no desktop; o avatar da conta fica no pé de ambos. */
export function Shell() {
  const { t } = useTranslation()

  return (
    <div className={styles.shell}>
      <header className={styles.top}>
        <p className={styles.brand}>{APP_NAME}</p>
      </header>

      <nav className={styles.nav} aria-label={t('nav.main')}>
        <ul className={styles.items}>
          {items.map(({ to, key, icon: Icon, end, soon }) => (
            <li key={key}>
              {soon ? (
                <span className={styles.link} aria-disabled="true">
                  <Icon className={styles.icon} aria-hidden="true" />
                  <span className={styles.text}>
                    {t(`nav.${key}`)}
                    <span className={styles.soon}>{t('routineSoon')}</span>
                  </span>
                </span>
              ) : (
                <NavLink to={to} end={end} className={styles.link}>
                  <Icon className={styles.icon} aria-hidden="true" />
                  <span className={styles.text}>{t(`nav.${key}`)}</span>
                </NavLink>
              )}
            </li>
          ))}
        </ul>
        <div className={styles.account}>
          <UserMenu />
        </div>
      </nav>

      <main className={styles.main}>
        <Outlet />
      </main>
    </div>
  )
}
