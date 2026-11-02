import { useTranslation } from 'react-i18next'
import { Button, EmptyState } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useSetModule } from '@/shared/modules/hooks'
import styles from './goals.module.css'

/** Metas desligado não é erro: é um convite para ligar, que explica que dá para desligar sem perder nada. */
export function ModuleInvite() {
  const { t } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const set = useSetModule('goals')
  return (
    <EmptyState
      title={t('module.title')}
      body={t('module.body')}
      action={
        <>
          <Button variant="primary" loading={set.isPending} onClick={() => set.mutate(true)}>
            {t('module.enable')}
          </Button>
          {set.error != null && (
            <p className={styles.error} role="alert">
              {errorMessage(tc, set.error)}
            </p>
          )}
        </>
      }
    />
  )
}
