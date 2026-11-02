import { Target } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Button } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useModuleEnabled, useSetModule } from '@/shared/modules/hooks'
import { useGoals } from './hooks'
import styles from './goals.module.css'

/**
 * O cartão de Metas em Casa. Desligado: um convite que explica que dá para desligar sem perder nada.
 * Ligado: um atalho com quantas metas estão ativas e outro para criar.
 */
export function GoalsHomeCard() {
  const { t } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const mod = useModuleEnabled('goals')
  const set = useSetModule('goals')
  const active = useGoals({ status: 'active' }, mod.on)

  if (!mod.isSuccess) return null

  if (!mod.on) {
    return (
      <div className={styles.homeOptional}>
        <p className={styles.homeName}>{t('home.offTitle')}</p>
        <p className={styles.homeHint}>{t('home.offHint')}</p>
        <Button loading={set.isPending} onClick={() => set.mutate(true)}>
          {t('module.enable')}
        </Button>
        {set.error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, set.error)}
          </p>
        )}
      </div>
    )
  }

  return (
    <div className={styles.homeRow}>
      <Link to="/goals" className={styles.homeLink}>
        <Target size={22} aria-hidden="true" />
        <span>
          <span className={styles.homeName}>{t('home.name')}</span>
          <span className={styles.homeHint}>{active.isSuccess ? t('home.active', { count: active.data.length }) : t('home.hint')}</span>
        </span>
      </Link>
      <Link to="/goals?new=1">{t('home.create')}</Link>
    </div>
  )
}
