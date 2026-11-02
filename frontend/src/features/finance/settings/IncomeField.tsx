import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Card, Skeleton, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useIncome, useIncomeActions, useSettings } from '../hooks'
import { centsToInput, interpretedHint, MAX_CENTS, parseMoney } from '../money'
import styles from '../finance.module.css'

/**
 * A renda mensal da própria pessoa. É opcional e privada: não existe consulta à renda do parceiro,
 * então esta tela nunca tem um valor de outra pessoa para mostrar.
 */
export function IncomeField() {
  const income = useIncome()
  if (income.isPending) return <Skeleton height="6rem" />
  const stored = income.data?.monthly_income_cents ?? null
  return <IncomeForm key={stored ?? 'none'} stored={stored} />
}

function IncomeForm({ stored }: { stored: number | null }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const actions = useIncomeActions()
  const settings = useSettings()
  const [text, setText] = useState(stored === null ? '' : centsToInput(stored))
  const cents = parseMoney(text)
  const invalid = text.trim() !== '' && (cents === null || cents <= 0 || cents > MAX_CENTS)
  const proportional = settings.data?.split_mode === 'proportional'
  const error = actions.save.error ?? actions.remove.error

  return (
    <Card title={t('income.title')}>
      <p className={styles.hint}>{t('income.privacy')}</p>
      <form
        className={styles.form}
        onSubmit={(e) => {
          e.preventDefault()
          if (cents !== null && !invalid && cents > 0) actions.save.mutate(cents)
        }}
        noValidate
      >
        <TextField
          label={t('income.label')}
          inputMode="decimal"
          autoComplete="off"
          placeholder="0,00"
          value={text}
          onChange={(e) => setText(e.target.value)}
          error={invalid ? t('form.amountInvalid') : undefined}
          hint={interpretedHint(t, cents, i18n.language)}
        />
        {proportional && <p className={styles.notice}>{t('income.notifyPartner')}</p>}
        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}
        <div className={styles.footer}>
          {stored !== null && (
            <Button variant="quiet" loading={actions.remove.isPending} onClick={() => actions.remove.mutate()}>
              {t('income.remove')}
            </Button>
          )}
          <Button type="submit" variant="primary" loading={actions.save.isPending} disabled={cents === null || invalid || cents === stored}>
            {t('income.save')}
          </Button>
        </div>
      </form>
    </Card>
  )
}
