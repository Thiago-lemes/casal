import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/design-system'
import { useMe } from '@/features/auth/hooks'
import { errorMessage } from '@/shared/api/errorMessage'
import { useCreateInvite, useHousehold } from './hooks'
import styles from './HomePage.module.css'

/** "Casa": quem faz parte do casal e o convite para a segunda pessoa. */
export function HomePage() {
  const { t } = useTranslation('household')
  const { t: tc } = useTranslation('common')
  const me = useMe().data
  const household = useHousehold().data
  const invite = useCreateInvite()
  const [copied, setCopied] = useState(false)

  if (!me || !household) return null
  const partner = household.members.find((m) => m.id !== me.id)

  async function copy(token: string) {
    try {
      await navigator.clipboard.writeText(token)
      setCopied(true)
    } catch {
      /* sem permissão de área de transferência: o código continua visível para copiar à mão */
    }
  }

  return (
    <div className={styles.page}>
      <h1 className={styles.title}>{household.members.map((m) => m.name).join(' & ')}</h1>

      {!partner && (
        <section className={styles.invite}>
          <h2 className={styles.subtitle}>{t('invite.title')}</h2>
          <p className={styles.lead}>{t('invite.lead')}</p>
          {!invite.data && (
            <Button variant="primary" loading={invite.isPending} onClick={() => invite.mutate()}>
              {t('invite.action')}
            </Button>
          )}
          {invite.data && (
            <>
              <code className={styles.token}>{invite.data.token}</code>
              <Button onClick={() => void copy(invite.data.token)}>
                {copied ? t('invite.copied') : t('invite.copy')}
              </Button>
            </>
          )}
          {invite.isError && (
            <p className={styles.error} role="alert">
              {errorMessage(tc, invite.error)}
            </p>
          )}
        </section>
      )}
    </div>
  )
}
