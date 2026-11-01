import { NavLink, Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { APP_NAME } from '@/shared/config'
import styles from './Shell.module.css'

const items = [
  { to: '/', key: 'today', end: true },
  { to: '/routine', key: 'routine', disabled: true },
  { to: '/home', key: 'home' },
  { to: '/more', key: 'more' },
] as const

/** Moldura do app: barra inferior no celular, coluna lateral no desktop. */
export function Shell() {
  const { t } = useTranslation()

  return (
    <div className={styles.shell}>
      <nav className={styles.nav} aria-label={t('nav.main')}>
        <p className={styles.brand}>{APP_NAME}</p>
        <ul className={styles.items}>
          {items.map((item) => (
            <li key={item.key}>
              {'disabled' in item ? (
                <span className={styles.link} aria-disabled="true">
                  {t(`nav.${item.key}`)}
                  <span className={styles.soon}>{t('routineSoon')}</span>
                </span>
              ) : (
                <NavLink to={item.to} end={'end' in item} className={styles.link}>
                  {t(`nav.${item.key}`)}
                </NavLink>
              )}
            </li>
          ))}
        </ul>
      </nav>
      <main className={styles.main}>
        <Outlet />
      </main>
    </div>
  )
}
