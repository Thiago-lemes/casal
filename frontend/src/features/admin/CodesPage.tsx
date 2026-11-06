import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Card, EmptyState, Skeleton, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useCodes, useCreateCode, useRevokeCode } from './hooks'
import { codeState, when } from './format'
import styles from './Admin.module.css'

/** Códigos de cadastro do beta. O valor só aparece uma vez, na criação. */
export function CodesPage() {
  const { t, i18n } = useTranslation('admin')
  const { t: tc } = useTranslation('common')
  const codes = useCodes()
  const create = useCreateCode()
  const revoke = useRevokeCode()
  const [note, setNote] = useState('')
  const [uses, setUses] = useState('1')
  const [days, setDays] = useState('14')

  function submit(e: FormEvent) {
    e.preventDefault()
    create.mutate({ note: note.trim(), maxUses: Number(uses) || 1, days: Number(days) || 14 }, { onSuccess: () => setNote('') })
  }

  return (
    <>
      <Card title={t('codes.new')}>
        <form className={styles.row} onSubmit={submit}>
          <TextField label={t('codes.note')} hint={t('codes.noteHint')} maxLength={100} value={note} onChange={(e) => setNote(e.target.value)} />
          <TextField label={t('codes.uses')} type="number" min={1} max={100} value={uses} onChange={(e) => setUses(e.target.value)} />
          <TextField label={t('codes.days')} type="number" min={1} max={90} value={days} onChange={(e) => setDays(e.target.value)} />
          <Button type="submit" variant="primary" loading={create.isPending}>
            {t('codes.create')}
          </Button>
        </form>
        {create.isError && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, create.error)}
          </p>
        )}
        {create.data && (
          <div role="status">
            <p className={styles.muted}>{t('codes.once')}</p>
            <code className={styles.secret}>{create.data.code}</code>
          </div>
        )}
      </Card>

      {codes.isPending && <Skeleton />}
      {codes.data && codes.data.length === 0 && <EmptyState title={t('codes.none')} />}
      <ul className={styles.list}>
        {codes.data?.map((c) => {
          const state = codeState(c)
          return (
            <li key={c.id} className={styles.item}>
              <div className={styles.itemHead}>
                <strong>{c.note || t('codes.noNote')}</strong>
                <span className={styles.state}>{t(`codes.state.${state}`)}</span>
              </div>
              <span className={styles.muted}>
                {t('codes.usage', { uses: c.uses, max: c.max_uses })} · {t('codes.until', { date: when(c.expires_at, i18n.language) })}
              </span>
              {state === 'active' && (
                <div>
                  <Button variant="quiet" loading={revoke.isPending && revoke.variables === c.id} onClick={() => revoke.mutate(c.id)}>
                    {t('codes.revoke')}
                  </Button>
                </div>
              )}
            </li>
          )
        })}
      </ul>
    </>
  )
}
