import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router-dom'
import { Button, Card, EmptyState, Skeleton, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useBlock, useHouseholdFile, useSearch, useSetModule, useUserFile } from './hooks'
import { when } from './format'
import styles from './Admin.module.css'

/** Busca por e-mail ou nome de quem pertence ao casal. */
export function HouseholdsPage() {
  const { t, i18n } = useTranslation('admin')
  const { t: tc } = useTranslation('common')
  const [input, setInput] = useState('')
  const [q, setQ] = useState('')
  const search = useSearch(q)

  function submit(e: FormEvent) {
    e.preventDefault()
    setQ(input.trim())
  }

  return (
    <>
      <form className={styles.row} onSubmit={submit}>
        <TextField label={t('households.search')} hint={t('households.searchHint')} value={input} onChange={(e) => setInput(e.target.value)} />
        <Button type="submit" variant="primary" disabled={input.trim().length < 2}>
          {t('households.searchButton')}
        </Button>
      </form>
      {search.isError && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, search.error)}
        </p>
      )}
      {search.data && search.data.length === 0 && <EmptyState title={t('households.none')} />}
      <ul className={styles.list}>
        {search.data?.map((h) => (
          <li key={h.id} className={styles.item}>
            <div className={styles.itemHead}>
              <Link to={`/admin/casais/${h.id}`}>{h.members.map((m) => m.name).join(' e ')}</Link>
              <span className={styles.muted}>{when(h.created_at, i18n.language)}</span>
            </div>
            <span className={styles.muted}>{h.members.map((m) => m.email).join(', ')}</span>
          </li>
        ))}
      </ul>
    </>
  )
}

/** Ficha do casal: só metadados e contagens; abrir a ficha fica registrado na auditoria. */
export function HouseholdPage() {
  const { id = '' } = useParams()
  const { t, i18n } = useTranslation('admin')
  const { t: tc } = useTranslation('common')
  const file = useHouseholdFile(id)
  const setModule = useSetModule(id)

  if (file.isPending) return <Skeleton />
  if (file.isError) return <EmptyState title={errorMessage(tc, file.error)} />
  const { household: h, modules } = file.data

  return (
    <>
      <Card title={h.members.map((m) => m.name).join(' e ')}>
        <dl className={styles.facts}>
          <div>
            <dt>{t('household.created')}</dt>
            <dd>{when(h.created_at, i18n.language)}</dd>
          </div>
          <div>
            <dt>{t('household.lastUsed')}</dt>
            <dd>{when(h.last_used_at, i18n.language)}</dd>
          </div>
          <div>
            <dt>{t('household.push')}</dt>
            <dd>{h.push_devices}</dd>
          </div>
        </dl>
      </Card>

      <Card title={t('household.people')}>
        <ul className={styles.list}>
          {h.members.map((m) => (
            <li key={m.id} className={styles.itemHead}>
              <Link to={`/admin/pessoas/${m.id}`}>{m.name}</Link>
              <span className={styles.muted}>{m.email}</span>
            </li>
          ))}
        </ul>
      </Card>

      <Card title={t('household.modules')}>
        <ul className={styles.list}>
          {modules.map((m) => (
            <li key={m.name}>
              <label className={styles.toggle}>
                <input
                  type="checkbox"
                  checked={m.enabled}
                  disabled={setModule.isPending}
                  onChange={(e) => setModule.mutate({ name: m.name, enabled: e.target.checked })}
                />
                {t(`modules.${m.name}`, { defaultValue: m.name })}
              </label>
            </li>
          ))}
        </ul>
        {setModule.isError && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, setModule.error)}
          </p>
        )}
      </Card>

      <Card title={t('household.counts')}>
        <p className={styles.muted}>{t('household.countsNote')}</p>
        <dl className={styles.facts}>
          {Object.entries(h.counts).map(([k, n]) => (
            <div key={k}>
              <dt>{t(`counts.${k}`, { defaultValue: k })}</dt>
              <dd>{n}</dd>
            </div>
          ))}
        </dl>
      </Card>
    </>
  )
}

/** Ficha da pessoa, com bloqueio e desbloqueio (não apaga nada). */
export function UserPage() {
  const { id = '' } = useParams()
  const { t, i18n } = useTranslation('admin')
  const { t: tc } = useTranslation('common')
  const file = useUserFile(id)
  const block = useBlock(id)
  const [confirming, setConfirming] = useState(false)

  if (file.isPending) return <Skeleton />
  if (file.isError) return <EmptyState title={errorMessage(tc, file.error)} />
  const u = file.data
  const blocked = u.blocked_at !== null

  return (
    <Card title={u.name}>
      <dl className={styles.facts}>
        <div>
          <dt>{t('user.email')}</dt>
          <dd>{u.email}</dd>
        </div>
        <div>
          <dt>{t('household.created')}</dt>
          <dd>{when(u.created_at, i18n.language)}</dd>
        </div>
        <div>
          <dt>{t('household.lastUsed')}</dt>
          <dd>{when(u.last_used_at, i18n.language)}</dd>
        </div>
        <div>
          <dt>{t('household.push')}</dt>
          <dd>{u.push_devices}</dd>
        </div>
        <div>
          <dt>{t('user.status')}</dt>
          <dd>{blocked ? t('user.blockedSince', { date: when(u.blocked_at, i18n.language) }) : t('user.active')}</dd>
        </div>
      </dl>
      {u.household_id && <Link to={`/admin/casais/${u.household_id}`}>{t('user.openHousehold')}</Link>}
      <p className={styles.muted}>{t('user.blockNote')}</p>
      {block.isError && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, block.error)}
        </p>
      )}
      {blocked ? (
        <Button loading={block.isPending} onClick={() => block.mutate(false)}>
          {t('user.unblock')}
        </Button>
      ) : confirming ? (
        <div className={styles.row}>
          <Button variant="danger" loading={block.isPending} onClick={() => block.mutate(true, { onSuccess: () => setConfirming(false) })}>
            {t('user.confirmBlock')}
          </Button>
          <Button variant="quiet" onClick={() => setConfirming(false)}>
            {tc('actions.cancel')}
          </Button>
        </div>
      ) : (
        <Button variant="danger" onClick={() => setConfirming(true)}>
          {t('user.block')}
        </Button>
      )}
    </Card>
  )
}
