import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { APP_NAME } from '@/shared/config'
import { useAcceptInvite, useCreateHousehold } from './hooks'
import styles from './SetupPage.module.css'

/** Primeiro contato: explica o app em uma frase e oferece os dois caminhos, criar o casal ou entrar com um convite. */
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
      <header className={styles.hero}>
        <h1 className={styles.title}>{t('setup.title', { app: APP_NAME })}</h1>
        <p className={styles.lead}>{t('setup.lead', { app: APP_NAME })}</p>
      </header>

      <section className={styles.block}>
        <h2 className={styles.subtitle}>{t('setup.createTitle')}</h2>
        <p className={styles.lead}>{t('setup.createLead')}</p>
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
        <p className={styles.lead}>{t('setup.joinLead')}</p>
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
