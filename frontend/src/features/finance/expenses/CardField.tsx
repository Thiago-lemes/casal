import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Card } from '../api'
import { useCardActions } from '../hooks'
import styles from '../finance.module.css'

const NEW = '__new__'

type Props = {
  cards: Card[]
  /** id do cartão escolhido; vazio = sem cartão */
  value: string
  onChange: (id: string) => void
  /** o cartão atual do gasto continua na lista mesmo arquivado */
  keep?: string
}

const day = (v: string) => {
  const n = Number(v)
  return v.trim() !== '' && Number.isInteger(n) && n >= 1 && n <= 31 ? n : undefined
}

/** Escolher o cartão do gasto (opcional) ou cadastrar um na hora. O cartão é pessoal: só os meus aparecem. */
export function CardField({ cards, value, onChange, keep }: Props) {
  const { t } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const id = useId()
  const actions = useCardActions()
  const [adding, setAdding] = useState(false)
  const [name, setName] = useState('')
  const [last4, setLast4] = useState('')
  const [closing, setClosing] = useState('')
  const [due, setDue] = useState('')

  const options = cards.filter((c) => !c.archived || c.id === keep)
  const last4Ok = last4 === '' || /^\d{4}$/.test(last4)
  const daysOk = (closing === '' || day(closing) !== undefined) && (due === '' || day(due) !== undefined)
  const canSave = name.trim() !== '' && last4Ok && daysOk

  function save() {
    if (!canSave) return
    actions.create.mutate(
      { name: name.trim(), last4: last4 || undefined, closing_day: day(closing), due_day: day(due) },
      {
        onSuccess: (card) => {
          onChange(card.id)
          setAdding(false)
          setName('')
          setLast4('')
          setClosing('')
          setDue('')
        },
      },
    )
  }

  return (
    <div className={styles.field}>
      <label htmlFor={id} className={styles.label}>
        {t('card.label')}
      </label>
      <select
        id={id}
        className={styles.select}
        value={adding ? NEW : value}
        onChange={(e) => {
          if (e.target.value === NEW) {
            setAdding(true)
            return
          }
          setAdding(false)
          onChange(e.target.value)
        }}
      >
        <option value="">{t('card.none')}</option>
        {options.map((c) => (
          <option key={c.id} value={c.id}>
            {c.last4 ? `${c.name} · ${c.last4}` : c.name}
            {c.archived ? ` (${t('categories.archivedTag')})` : ''}
          </option>
        ))}
        <option value={NEW}>{t('card.new')}</option>
      </select>

      {adding && (
        <div className={styles.form}>
          <TextField label={t('card.name')} value={name} onChange={(e) => setName(e.target.value)} maxLength={40} autoComplete="off" required />
          <div className={styles.row2}>
            <TextField
              label={t('card.last4')}
              inputMode="numeric"
              maxLength={4}
              value={last4}
              onChange={(e) => setLast4(e.target.value.replace(/\D/g, ''))}
              error={last4Ok ? undefined : t('card.last4Invalid')}
            />
            <TextField label={t('card.closingDay')} inputMode="numeric" maxLength={2} value={closing} onChange={(e) => setClosing(e.target.value.replace(/\D/g, ''))} />
            <TextField label={t('card.dueDay')} inputMode="numeric" maxLength={2} value={due} onChange={(e) => setDue(e.target.value.replace(/\D/g, ''))} />
          </div>
          <p className={styles.hint}>{t('card.hint')}</p>
          {actions.create.error != null && (
            <p className={styles.error} role="alert">
              {errorMessage(tc, actions.create.error)}
            </p>
          )}
          <div className={styles.footer}>
            <Button variant="quiet" onClick={() => setAdding(false)}>
              {tc('actions.cancel')}
            </Button>
            <Button variant="primary" loading={actions.create.isPending} disabled={!canSave} onClick={save}>
              {t('card.save')}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
