import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Card, ChoiceGroup, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { SettingsView, Split } from '../api'
import { SplitEditor } from '../expenses/SplitEditor'
import { useSettings, useSettingsActions } from '../hooks'
import { splitLabel } from '../money'
import { equalSplit } from '../split'
import type { Ctx } from '../types'
import { nameOf } from '../types'
import { ConsentDialog } from './ConsentDialog'
import styles from '../finance.module.css'

type Mode = SettingsView['split_mode']

/** Como os novos gastos do casal são divididos por padrão. Mudar a divisão não reescreve os gastos antigos. */
export function DivisionSettings({ ctx }: { ctx: Ctx }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const settings = useSettings()

  if (settings.isPending) return <Skeleton height="8rem" />
  if (settings.isError) {
    return (
      <Card title={t('division.title')}>
        <p className={styles.error} role="alert">
          {errorMessage(tc, settings.error)}
        </p>
        <Button onClick={() => void settings.refetch()}>{tc('actions.tryAgain')}</Button>
      </Card>
    )
  }
  // o formulário nasce do que o servidor devolveu; ao mudar a divisão por fora (ex.: parceiro retirou), ele se refaz
  const key = `${settings.data.split_mode}:${JSON.stringify(settings.data.fixed_split)}:${settings.data.proportional.consented_by.join(',')}`
  return <DivisionForm key={key} ctx={ctx} settings={settings.data} lang={i18n.language} />
}

function DivisionForm({ ctx, settings, lang }: { ctx: Ctx; settings: SettingsView; lang: string }) {
  const { t } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const actions = useSettingsActions()
  const [mode, setMode] = useState<Mode>(settings.split_mode)
  const [fixed, setFixed] = useState<Split>(settings.fixed_split ?? equalSplit(ctx.people))
  const [consenting, setConsenting] = useState(false)

  const p = settings.proportional
  const iConsented = p.consented_by.includes(ctx.me.id)
  const partnerConsented = ctx.partner ? p.consented_by.includes(ctx.partner.id) : false
  const options: { value: Mode; label: string }[] = [
    { value: 'equal', label: t('division.equal') },
    { value: 'fixed', label: t('division.fixed') },
    ...(p.available || settings.split_mode === 'proportional' ? [{ value: 'proportional' as const, label: t('division.proportional') }] : []),
  ]
  const dirty = mode !== settings.split_mode || (mode === 'fixed' && JSON.stringify(fixed) !== JSON.stringify(settings.fixed_split))

  const effective = ctx.people
    .map((person) => `${person.id === ctx.me.id ? t('form.me') : person.name} ${splitLabel(settings.effective_split[person.id] ?? 0, lang)}`)
    .join(' / ')

  return (
    <Card title={t('division.title')}>
      <p>{t('division.today', { split: effective })}</p>
      <p className={styles.hint}>{t('division.help')}</p>

      <form
        className={styles.form}
        onSubmit={(e) => {
          e.preventDefault()
          actions.save.mutate({ split_mode: mode, ...(mode === 'fixed' ? { fixed_split: fixed } : {}) })
        }}
      >
        <ChoiceGroup legend={t('division.legend')} options={options} value={mode} onChange={setMode} />
        {mode === 'fixed' && ctx.partner && <SplitEditor people={ctx.people} value={fixed} onChange={setFixed} />}
        {mode === 'proportional' && p.split && (
          <p className={styles.hint}>
            {t('division.result', {
              split: ctx.people.map((x) => `${x.id === ctx.me.id ? t('form.me') : x.name} ${splitLabel(p.split?.[x.id] ?? 0, lang)}`).join(' / '),
            })}
          </p>
        )}
        {actions.save.error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, actions.save.error)}
          </p>
        )}
        <div>
          <Button type="submit" variant="primary" loading={actions.save.isPending} disabled={!dirty}>
            {t('division.save')}
          </Button>
        </div>
      </form>

      {ctx.partner && (
        <section className={styles.form} aria-label={t('division.proportional')}>
          <h3 style={{ fontSize: 'var(--text-md)' }}>{t('division.proportional')}</h3>
          <p className={styles.hint}>{t('division.proportionalHelp')}</p>
          {/* só quem informou a renda, nunca o valor */}
          <ul className={styles.hint}>
            <li>{p.income_set[ctx.me.id] ? t('division.incomeMineSet') : t('division.incomeMineMissing')}</li>
            <li>
              {p.income_set[ctx.partner.id]
                ? t('division.incomePartnerSet', { name: nameOf(ctx, ctx.partner.id) })
                : t('division.incomePartnerMissing', { name: nameOf(ctx, ctx.partner.id) })}
            </li>
            <li>{iConsented ? t('division.consentMine') : t('division.consentMineMissing')}</li>
            <li>
              {partnerConsented
                ? t('division.consentPartner', { name: ctx.partner.name })
                : t('division.consentPartnerMissing', { name: ctx.partner.name })}
            </li>
          </ul>
          {(actions.withdraw.error ?? actions.consent.error) != null && !consenting && (
            <p className={styles.error} role="alert">
              {errorMessage(tc, actions.withdraw.error ?? actions.consent.error)}
            </p>
          )}
          <div>
            {iConsented ? (
              <Button loading={actions.withdraw.isPending} onClick={() => actions.withdraw.mutate()}>
                {t('division.withdraw')}
              </Button>
            ) : (
              <Button onClick={() => setConsenting(true)}>{t('division.consent')}</Button>
            )}
          </div>
        </section>
      )}

      {consenting && (
        <ConsentDialog
          pending={actions.consent.isPending}
          error={actions.consent.error}
          onClose={() => setConsenting(false)}
          onConfirm={() => actions.consent.mutate(undefined, { onSuccess: () => setConsenting(false) })}
        />
      )}
    </Card>
  )
}
