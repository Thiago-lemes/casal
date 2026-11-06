import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { Button, Card } from '@/design-system'
import type { Step } from './progress'
import styles from './FirstSteps.module.css'

type Props = {
  steps: Step[]
  onDismiss: () => void
  onNewReminder: () => void
  onEnablePush: () => void
  pushPending?: boolean
  /** texto de erro de ativar os avisos, já traduzido */
  pushError?: string | null
  /** no iPhone fora da tela inicial, o passo explica como instalar em vez de oferecer o botão */
  pushNeedsInstall?: boolean
}

/** A lista curta que acompanha a pessoa nos primeiros dias; some sozinha quando tudo está feito, ou ao dispensar. */
export function FirstSteps({ steps, onDismiss, onNewReminder, onEnablePush, pushPending, pushError, pushNeedsInstall }: Props) {
  const { t } = useTranslation('onboarding')
  const { t: tp } = useTranslation('push')
  const navigate = useNavigate()
  const done = steps.filter((s) => s.done).length

  function action(id: Step['id']) {
    const label = t(`steps.${id}.action`)
    switch (id) {
      case 'city':
        return <Button onClick={() => navigate('/profile')}>{label}</Button>
      case 'invite':
      case 'modules':
        return <Button onClick={() => navigate('/home')}>{label}</Button>
      case 'reminder':
        return <Button onClick={onNewReminder}>{label}</Button>
      case 'push':
        return pushNeedsInstall ? null : (
          <Button loading={pushPending} onClick={onEnablePush}>
            {label}
          </Button>
        )
    }
  }

  return (
    <Card
      title={t('title')}
      className={styles.card}
      aside={<span className={styles.count}>{t('progress', { done, total: steps.length })}</span>}
    >
      <ol className={styles.list}>
        {steps.map((s) => (
          <li key={s.id} className={styles.step} data-done={s.done || undefined}>
            <span className={styles.mark} aria-hidden={!s.done}>
              {s.done && <Check size={16} aria-label={t('done')} />}
            </span>
            <div className={styles.text}>
              <p className={styles.name}>{t(`steps.${s.id}.title`)}</p>
              {!s.done && (
                <>
                  <p className={styles.hint}>
                    {s.id === 'push' && pushNeedsInstall ? t('steps.push.hintInstall') : t(`steps.${s.id}.hint`)}
                  </p>
                  {s.id === 'push' && pushNeedsInstall && (
                    <ol className={styles.install}>
                      <li>{tp('install.step1')}</li>
                      <li>{tp('install.step2')}</li>
                      <li>{tp('install.step3')}</li>
                    </ol>
                  )}
                  {s.id === 'push' && pushError && (
                    <p className={styles.error} role="alert">
                      {pushError}
                    </p>
                  )}
                </>
              )}
            </div>
            {!s.done && <div className={styles.action}>{action(s.id)}</div>}
          </li>
        ))}
      </ol>
      <div>
        <Button variant="quiet" aria-label={t('dismissLabel')} onClick={onDismiss}>
          {t('dismiss')}
        </Button>
      </div>
    </Card>
  )
}
