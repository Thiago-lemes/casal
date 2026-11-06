import { CalendarClock, House, LayoutDashboard, ListChecks, Target, Wallet } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { NavLink, Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useHousehold } from '@/features/household/hooks'
import { Logo } from '@/design-system'
import { APP_NAME } from '@/shared/config'
import styles from './Shell.module.css'
import { UserMenu } from './UserMenu'

type Item = { to: string; key: string; icon: LucideIcon; end?: boolean; /** no celular sai da barra e vai para o menu do avatar */ desktopOnly?: boolean }

const items: Item[] = [
  { to: '/', key: 'today', icon: LayoutDashboard, end: true },
  { to: '/routine', key: 'routine', icon: CalendarClock, desktopOnly: true },
  { to: '/tasks', key: 'tasks', icon: ListChecks },
  { to: '/home', key: 'home', icon: House },
]

/** Módulo opcional: sempre na navegação (antes de Casa); o casal liga dentro da página. */
const finance: Item = { to: '/finance', key: 'finance', icon: Wallet }

/** Módulo opcional: como a Semana, no celular sai da barra e vai para o menu do avatar. */
const goals: Item = { to: '/goals', key: 'goals', icon: Target, desktopOnly: true }

/** Moldura do app: barra inferior no celular, trilho no desktop; o avatar da conta fica no topo (celular) ou no pé do trilho (desktop). */
export function Shell() {
  const { t } = useTranslation()
  const household = useHousehold().data
  // Finanças e Metas ficam sempre no menu (havendo casal); desligados, a página mostra o convite para ligar.
  const nav = [...items.slice(0, -1), ...(household ? [finance, goals] : []), items[items.length - 1]]

  return (
    <div className={styles.shell}>
      <header className={styles.top}>
        {/* celular: o avatar fica no topo, ao lado do nome; no desktop ele está no pé do trilho */}
        <div className={styles.topAccount}>
          <UserMenu side="bottom" />
        </div>
        {/* celular: fundo claro, cores da marca; desktop: trilho laranja, versão para laranja */}
        <Logo className={styles.brandMobile} title={APP_NAME} />
        <Logo className={styles.brandRail} tone="onDark" title={APP_NAME} />
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
