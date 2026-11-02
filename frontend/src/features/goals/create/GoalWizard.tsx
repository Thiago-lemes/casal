import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose } from '@/design-system'
import { todayIn } from '@/shared/lib/dates'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Goal, Kind } from '../api'
import { useGoalActions } from '../hooks'
import type { FinanceOptions, Me } from '../types'
import { allowedScopes, buildInput, emptyForm, validate } from './form'
import type { FieldErrors, Form } from './form'
import { GoalFields } from './TypeFields'
import { VisibilityChoice } from './VisibilityChoice'
import styles from '../goals.module.css'

type Step = 'kind' | 'scope' | 'fields' | 'visibility'

const KINDS: Kind[] = ['habit', 'shared', 'financial', 'project']

/**
 * Os passos dependem do que já foi escolhido: o que só tem uma opção válida (hábito é só pessoal,
 * meta do casal só do casal) não vira pergunta, e "quem pode ver" só existe para meta pessoal.
 */
export function stepsFor(form: Form): Step[] {
  const steps: Step[] = ['kind']
  if (form.kind && allowedScopes(form.kind).length > 1) steps.push('scope')
  steps.push('fields')
  if (form.scope === 'personal') steps.push('visibility')
  return steps
}

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  me: Me
  finance?: FinanceOptions
  onCreated: (goal: Goal) => void
}

/** Assistente de criação: tipo, de quem é, os campos do tipo e (só pessoal) quem pode ver. */
export function GoalWizard({ open, onOpenChange, me, finance, onCreated }: Props) {
  const { t } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const { create } = useGoalActions()
  const [form, setForm] = useState<Form>(emptyForm)
  const [step, setStep] = useState<Step>('kind')
  const [errors, setErrors] = useState<FieldErrors>({})
  const today = todayIn(me.timezone)

  const steps = stepsFor(form)
  const index = steps.indexOf(step)
  const last = index === steps.length - 1
  const set = (patch: Partial<Form>) => setForm((f) => ({ ...f, ...patch }))

  function pickKind(kind: Kind) {
    const scopes = allowedScopes(kind)
    setForm((f) => ({ ...f, kind, scope: scopes.length === 1 ? scopes[0] : f.scope && scopes.includes(f.scope) ? f.scope : null }))
    setStep(scopes.length === 1 ? 'fields' : 'scope')
  }

  function pickScope(scope: 'household' | 'personal') {
    set({ scope, source: 'contributions', categoryId: '', stepTasks: false })
    setStep('fields')
  }

  function back() {
    create.reset()
    if (index > 0) setStep(steps[index - 1])
  }

  function next(e: FormEvent) {
    e.preventDefault()
    if (step === 'fields') {
      const found = validate(form, today)
      setErrors(found)
      if (Object.keys(found).length > 0) return
    }
    if (!last) return setStep(steps[index + 1])
    create.mutate(buildInput(form), {
      onSuccess: (goal) => {
        onOpenChange(false)
        onCreated(goal)
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t('wizard.title')}>
      <form className={styles.form} onSubmit={next} noValidate>
        <p className={styles.wizardSteps}>{t('wizard.step', { n: index + 1, total: steps.length })}</p>

        {step === 'kind' && (
          <div className={styles.form} role="group" aria-label={t('wizard.kindLegend')}>
            {KINDS.map((k) => (
              <button key={k} type="button" className={styles.choice} aria-pressed={form.kind === k} onClick={() => pickKind(k)}>
                <strong>{t(`kind.${k}`)}</strong>
                <span>{t(`kindHint.${k}`)}</span>
              </button>
            ))}
          </div>
        )}

        {step === 'scope' && (
          <div className={styles.form} role="group" aria-label={t('wizard.scopeLegend')}>
            {(['personal', 'household'] as const).map((s) => (
              <button key={s} type="button" className={styles.choice} aria-pressed={form.scope === s} onClick={() => pickScope(s)}>
                <strong>{t(`scope.${s}`)}</strong>
                <span>{t(`scopeHint.${s}`)}</span>
              </button>
            ))}
          </div>
        )}

        {step === 'fields' && (
          <>
            <p className={styles.hint}>{t(`scopeLine.${form.scope ?? 'personal'}`, { kind: form.kind ? t(`kind.${form.kind}`) : '' })}</p>
            <GoalFields form={form} set={set} errors={errors} today={today} finance={finance} />
          </>
        )}

        {step === 'visibility' && <VisibilityChoice value={form.visibility} onChange={(visibility) => set({ visibility })} />}

        {create.error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, create.error)}
          </p>
        )}

        <div className={styles.footer}>
          {index === 0 ? (
            <DialogClose asChild>
              <Button variant="quiet">{tc('actions.cancel')}</Button>
            </DialogClose>
          ) : (
            <Button variant="quiet" onClick={back}>
              {t('wizard.back')}
            </Button>
          )}
          {step !== 'kind' && step !== 'scope' && (
            <Button type="submit" variant="primary" loading={create.isPending}>
              {last ? t('wizard.create') : t('wizard.next')}
            </Button>
          )}
        </div>
      </form>
    </Dialog>
  )
}
