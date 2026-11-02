import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Avatar, Button, Card, Dialog, DialogClose, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { CITIES, cityForZone, cityName, deviceZone } from './cities'
import styles from './ProfilePage.module.css'

type Mutation<V> = { mutate: (vars: V, o?: { onSuccess?: () => void }) => void; isPending: boolean; error: unknown; isSuccess: boolean }

type Props = {
  me: { name: string; email: string; timezone: string }
  /** vem de `useUpdateProfile`; o fuso é o nome IANA da cidade escolhida */
  update: Mutation<{ name?: string; timezone?: string }>
  /** vem de `useLeaveHousehold`; sem casal, a pessoa não tem o que sair */
  leave?: Mutation<void>
}

export function ProfilePage({ me, update, leave }: Props) {
  const { t, i18n } = useTranslation('profile')
  const { t: tc } = useTranslation('common')
  const [name, setName] = useState(me.name)
  const [zone, setZone] = useState(me.timezone)
  const [confirming, setConfirming] = useState(false)

  const known = cityForZone(zone)
  const device = deviceZone()
  const deviceCity = device ? cityForZone(device) : undefined

  function saveName(e: FormEvent) {
    e.preventDefault()
    update.mutate({ name: name.trim() })
  }

  function saveZone(e: FormEvent) {
    e.preventDefault()
    update.mutate({ timezone: zone })
  }

  return (
    <div className={styles.page}>
      <header className={styles.hero}>
        <Avatar name={me.name} className={styles.avatar} />
        <div>
          <h1 className={styles.title}>{me.name}</h1>
          <p className={styles.email}>{me.email}</p>
        </div>
      </header>

      <Card title={t('name.title')}>
        <form className={styles.form} onSubmit={saveName}>
          <TextField label={t('name.label')} value={name} onChange={(e) => setName(e.target.value)} required maxLength={80} />
          <div>
            <Button type="submit" variant="primary" loading={update.isPending} disabled={!name.trim() || name.trim() === me.name}>
              {t('save')}
            </Button>
          </div>
        </form>
      </Card>

      <Card title={t('city.title')}>
        <form className={styles.form} onSubmit={saveZone}>
          <p className={styles.lead}>{t('city.lead')}</p>
          <div className={styles.select}>
            <label htmlFor="city" className={styles.label}>
              {t('city.label')}
            </label>
            <select
              id="city"
              value={zone}
              onChange={(e) => setZone(e.target.value)}
              className={styles.control}
            >
              {/* um fuso que não está na lista (ex.: escolhido antes) continua selecionável */}
              {!known && <option value={zone}>{zone}</option>}
              {CITIES.map((city) => (
                <option key={city.id} value={city.tz}>
                  {cityName(city, i18n.language)}
                </option>
              ))}
            </select>
          </div>
          {deviceCity && device !== zone && (
            <Button variant="quiet" onClick={() => setZone(device!)}>
              {t('city.useDevice', { city: cityName(deviceCity, i18n.language) })}
            </Button>
          )}
          <div>
            <Button type="submit" variant="primary" loading={update.isPending} disabled={zone === me.timezone}>
              {t('save')}
            </Button>
          </div>
        </form>
      </Card>

      {update.error != null && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, update.error)}
        </p>
      )}
      {update.isSuccess && !update.isPending && update.error == null && (
        <p className={styles.saved} role="status">
          {t('saved')}
        </p>
      )}

      {leave && (
        <Card title={t('leave.title')}>
          <p className={styles.lead}>{t('leave.lead')}</p>
          <div>
            <Button variant="danger" onClick={() => setConfirming(true)}>
              {t('leave.action')}
            </Button>
          </div>
        </Card>
      )}

      {leave && (
        <Dialog open={confirming} onOpenChange={setConfirming} title={t('leave.confirmTitle')}>
          <div className={styles.form}>
            <ul className={styles.consequences}>
              <li>{t('leave.keep')}</li>
              <li>{t('leave.takes')}</li>
              <li>{t('leave.cancels')}</li>
              <li>{t('leave.final')}</li>
            </ul>
            {leave.error != null && (
              <p className={styles.error} role="alert">
                {errorMessage(tc, leave.error)}
              </p>
            )}
            <div className={styles.footer}>
              <DialogClose asChild>
                <Button variant="quiet">{tc('actions.cancel')}</Button>
              </DialogClose>
              <Button variant="danger" loading={leave.isPending} onClick={() => leave.mutate(undefined, { onSuccess: () => setConfirming(false) })}>
                {t('leave.confirm')}
              </Button>
            </div>
          </div>
        </Dialog>
      )}
    </div>
  )
}
