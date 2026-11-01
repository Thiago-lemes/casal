import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useAcceptInvite, useCreateHousehold } from './hooks'
import styles from './SetupPage.module.css'

/** Primeiro contato: ou cria o casal, ou entra com um convite. */
export function SetupPage() {
  const { t } = useTranslation('household')
  const { t: tc } = useTranslation('common')
  const create = useCreateHousehold()
  const accept = useAcceptInvite()
  const [token, setToken] = useState('')

  function join(e: FormEvent) {
    e.preventDefault()
    accept.mutate(token.trim())
  }

  return (
    <div className={styles.page}>
      <section className={styles.block}>
        <h1 className={styles.title}>{t('setup.title')}</h1>
        <p className={styles.lead}>{t('setup.lead')}</p>
        <Button variant="primary" loading={create.isPending} onClick={() => create.mutate()}>
          {t('setup.create')}
        </Button>
        {create.isError && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, create.error)}
          </p>
        )}
      </section>

      <form className={styles.block} onSubmit={join}>
        <h2 className={styles.subtitle}>{t('setup.joinTitle')}</h2>
        <TextField label={t('setup.token')} value={token} onChange={(e) => setToken(e.target.value)} required />
        {accept.isError && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, accept.error)}
          </p>
        )}
        <Button type="submit" loading={accept.isPending}>
          {t('setup.join')}
        </Button>
      </form>
    </div>
  )
}
