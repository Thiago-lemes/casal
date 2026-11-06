import { useTranslation } from 'react-i18next'
import { Navigate, NavLink, Outlet } from 'react-router-dom'
import { Button, LoadingScreen } from '@/design-system'
import { useAdminLogout, useAdminMe } from './hooks'
import styles from './Admin.module.css'

/** Só passa quem tem sessão do painel; sem ela vai para o login do painel. */
export function RequireAdmin() {
  const { t } = useTranslation('admin')
  const me = useAdminMe()
  if (me.isPending) return <LoadingScreen label={t('loading')} />
  if (!me.data) return <Navigate to="/admin/login" replace />
  return <Outlet />
}

export function AdminShell() {
  const { t } = useTranslation('admin')
  const me = useAdminMe().data
  const logout = useAdminLogout()
  const links = [
    ['casais', t('nav.households')],
    ['convites', t('nav.codes')],
    ['feedback', t('nav.feedback')],
    ['auditoria', t('nav.audit')],
  ] as const

  return (
    <div className={styles.shell}>
      <header className={styles.top}>
        <span className={styles.brand}>{t('brand')}</span>
        <nav className={styles.nav} aria-label={t('nav.label')}>
          {links.map(([to, label]) => (
            <NavLink key={to} to={`/admin/${to}`}>
              {label}
            </NavLink>
          ))}
        </nav>
        <div className={styles.who}>
          <span>{me?.email}</span>
          <Button variant="quiet" loading={logout.isPending} onClick={() => logout.mutate()}>
            {t('signOut')}
          </Button>
        </div>
      </header>
      <main className={styles.main}>
        <Outlet />
      </main>
    </div>
  )
}
