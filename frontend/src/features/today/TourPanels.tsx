import { Check, Copy } from 'lucide-react'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/design-system'
import { useUpdateProfile } from '@/features/auth/hooks'
import { useCreateInvite } from '@/features/household/hooks'
import { isValidZone, zoneLabel } from '@/features/profile/timezones'
import { ZonePicker } from '@/features/profile/ZonePicker'
import { errorMessage } from '@/shared/api/errorMessage'
import styles from './TourPanels.module.css'

/** Erro curto de uma mutação, anunciado por leitores de tela. */
function Failure({ error }: { error: unknown }) {
  const { t: tc } = useTranslation('common')
  if (error == null) return null
  return (
    <p className={styles.error} role="alert">
      {errorMessage(tc, error)}
    </p>
  )
}

function Done({ children }: { children: string }) {
  return (
    <p className={styles.done} role="status">
      <Check size={18} aria-hidden="true" /> {children}
    </p>
  )
}

/** Mostra o fuso que o app tem e deixa confirmar com um toque, ou trocar ali mesmo. */
export function CityPanel({ timezone, done, onChecked }: { timezone: string; done: boolean; onChecked: () => void }) {
  const { t, i18n } = useTranslation('onboarding')
  const update = useUpdateProfile()
  const [changing, setChanging] = useState(false)
  const [zone, setZone] = useState(timezone)

  if (done) return <Done>{t('panels.city.done')}</Done>

  function save(e: FormEvent) {
    e.preventDefault()
    update.mutate({ timezone: zone }, { onSuccess: onChecked })
  }

  if (!changing) {
    return (
      <>
        <p>
          {t('panels.city.current')} <strong>{zoneLabel(timezone, i18n.language)}</strong>
        </p>
        <div className={styles.row}>
          <Button variant="primary" onClick={onChecked}>
            {t('panels.city.right')}
          </Button>
          <Button onClick={() => setChanging(true)}>{t('panels.city.change')}</Button>
        </div>
      </>
    )
  }

  return (
    <form className={styles.form} onSubmit={save}>
      <ZonePicker label={t('panels.city.label')} value={zone} onChange={setZone} />
      <Button type="submit" variant="primary" loading={update.isPending} disabled={zone === timezone || !isValidZone(zone)}>
        {t('panels.city.save')}
      </Button>
      <Failure error={update.error} />
    </form>
  )
}

/** Gera o código do convite e já o deixa copiar. `onGenerated` avisa o tour para comemorar. */
export function InvitePanel({ onGenerated }: { onGenerated: () => void }) {
  const { t } = useTranslation('onboarding')
  const invite = useCreateInvite()
  const [copied, setCopied] = useState(false)

  async function copy(token: string) {
    try {
      await navigator.clipboard.writeText(token)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2500)
    } catch {
      /* sem permissão de área de transferência: o código continua visível para copiar à mão */
    }
  }

  if (!invite.data) {
    return (
      <>
        <Button variant="primary" loading={invite.isPending} onClick={() => invite.mutate(undefined, { onSuccess: onGenerated })}>
          {t('panels.invite.generate')}
        </Button>
        <Failure error={invite.error} />
      </>
    )
  }
  return (
    <>
      <code className={styles.token}>{invite.data.token}</code>
      <p>{t('panels.invite.lead')}</p>
      <Button variant="primary" onClick={() => void copy(invite.data.token)}>
        {copied ? <Check size={18} aria-hidden="true" /> : <Copy size={18} aria-hidden="true" />}
        {copied ? t('panels.invite.copied') : t('panels.invite.copy')}
      </Button>
    </>
  )
}

/** Ativar os avisos; no iPhone fora da tela inicial, o passo a passo de instalação. */
export function PushPanel({
  done,
  needsInstall,
  pending,
  error,
  onEnable,
}: {
  done: boolean
  needsInstall: boolean
  pending: boolean
  error: string | null
  onEnable: () => void
}) {
  const { t } = useTranslation('onboarding')
  const { t: tp } = useTranslation('push')
  if (done) return <Done>{t('panels.push.done')}</Done>
  return (
    <>
      {needsInstall ? (
        <>
          <p>{t('steps.push.bodyInstall')}</p>
          <ol className={styles.install}>
            <li>{tp('install.step1')}</li>
            <li>{tp('install.step2')}</li>
            <li>{tp('install.step3')}</li>
          </ol>
        </>
      ) : (
        <Button variant="primary" loading={pending} onClick={onEnable}>
          {t('panels.push.action')}
        </Button>
      )}
      {error && (
        <p className={styles.error} role="alert">
          {error}
        </p>
      )}
    </>
  )
}
