import { Bell, BellRing } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import { PushError, usePush } from './hooks'
import styles from './PushPrompt.module.css'

const KEY = 'casal.push.dismissed'

function wasDismissed() {
  try {
    return localStorage.getItem(KEY) === '1'
  } catch {
    return false
  }
}

/** Convite para ativar os avisos; no iPhone, o passo a passo para instalar o app antes. `ready` deixa quem usa escolher o momento. */
export function PushPrompt({ ready = true }: { ready?: boolean }) {
  const { t } = useTranslation('push')
  const { t: tc } = useTranslation('common')
  const { status, enable } = usePush()
  const [dismissed, setDismissed] = useState(wasDismissed)

  if (!ready || dismissed || (status !== 'off' && status !== 'install')) return null

  function dismiss() {
    try {
      localStorage.setItem(KEY, '1')
    } catch {
      /* sem armazenamento: some só nesta visita */
    }
    setDismissed(true)
  }

  const error =
    enable.error instanceof PushError
      ? t(`error.${enable.error.reason}`)
      : enable.error instanceof ApiError
        ? errorMessage(tc, enable.error)
        : enable.isError
          ? errorMessage(tc, enable.error)
          : null

  return (
    <section className={styles.prompt} aria-labelledby="push-title">
      <span className={styles.badge} aria-hidden="true">
        {status === 'install' ? <BellRing size={22} /> : <Bell size={22} />}
      </span>

      <div className={styles.text}>
        <h2 id="push-title" className={styles.title}>
          {status === 'install' ? t('install.title') : t('banner.title')}
        </h2>
        <p className={styles.body}>{status === 'install' ? t('install.body') : t('banner.body')}</p>
        {status === 'install' && (
          <ol className={styles.steps}>
            <li>{t('install.step1')}</li>
            <li>{t('install.step2')}</li>
            <li>{t('install.step3')}</li>
          </ol>
        )}
        {error && (
          <p className={styles.error} role="alert">
            {error}
          </p>
        )}
      </div>

      <div className={styles.actions}>
        {status === 'off' && (
          <Button variant="primary" loading={enable.isPending} onClick={() => enable.mutate()}>
            {t('banner.enable')}
          </Button>
        )}
        <Button variant="quiet" onClick={dismiss}>
          {t('banner.later')}
        </Button>
      </div>
    </section>
  )
}
