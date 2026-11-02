import { CalendarClock, House, LayoutDashboard, ListChecks, Wallet } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { NavLink, Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useHousehold } from '@/features/household/hooks'
import { APP_NAME } from '@/shared/config'
import { useModuleEnabled } from '@/shared/modules/hooks'
import styles from './Shell.module.css'
import { UserMenu } from './UserMenu'

type Item = { to: string; key: string; icon: LucideIcon; end?: boolean; /** no celular sai da barra e vai para o menu do avatar */ desktopOnly?: boolean }

const items: Item[] = [
  { to: '/', key: 'today', icon: LayoutDashboard, end: true },
  { to: '/routine', key: 'routine', icon: CalendarClock, desktopOnly: true },
  { to: '/tasks', key: 'tasks', icon: ListChecks },
  { to: '/home', key: 'home', icon: House },
]

/** Módulo opcional: só entra na navegação quando o casal o liga (antes de Casa). */
const finance: Item = { to: '/finance', key: 'finance', icon: Wallet }

/** Moldura do app: barra inferior no celular, trilho no desktop; o avatar da conta fica no topo (celular) ou no pé do trilho (desktop). */
export function Shell() {
  const { t } = useTranslation()
  const household = useHousehold().data
  const financeOn = useModuleEnabled('finance', Boolean(household)).on
  const nav = financeOn ? [...items.slice(0, -1), finance, items[items.length - 1]] : items

  return (
    <div className={styles.shell}>
      <header className={styles.top}>
        {/* celular: o avatar fica no topo, ao lado do nome; no desktop ele está no pé do trilho */}
        <div className={styles.topAccount}>
          <UserMenu side="bottom" />
        </div>
        <p className={styles.brand}>{APP_NAME}</p>
      </header>

      <nav className={styles.nav} aria-label={t('nav.main')}>
        <ul className={styles.items}>
          {nav.map(({ to, key, icon: Icon, end, desktopOnly }) => (
            <li key={key} className={desktopOnly ? styles.desktopOnly : undefined}>
              <NavLink to={to} end={end} className={styles.link}>
                <Icon className={styles.icon} aria-hidden="true" />
                <span className={styles.text}>{t(`nav.${key}`)}</span>
              </NavLink>
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
