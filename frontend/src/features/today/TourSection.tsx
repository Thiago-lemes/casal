import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Tour } from '@/design-system'
import type { TourStep } from '@/design-system'
import { planTour, readSaved, RESTART_EVENT, writeSaved } from '@/features/onboarding/progress'
import type { TourStepId } from '@/features/onboarding/progress'
import { PushError, usePush } from '@/features/push/hooks'
import { PushPrompt } from '@/features/push/PushPrompt'
import { useReminders } from '@/features/reminders/hooks'
import { errorMessage } from '@/shared/api/errorMessage'
import { CityPanel, InvitePanel, PushPanel } from './TourPanels'

/** O `data-tour` de cada passo: o que fica destacado na tela. */
const TARGETS: Partial<Record<TourStepId, string>> = {
  nav: 'nav',
  reminder: 'new-reminder',
  list: 'today-list',
  city: 'account',
  invite: 'home',
  push: 'account',
}

type Props = {
  me: { id: string; timezone: string }
  hasPartner: boolean
  /** o compositor de lembretes está aberto: o tour espera, sem perder o passo */
  composing: boolean
}

/**
 * O tour de boas-vindas em Hoje. Aponta para as partes reais da tela e, nos passos que pedem ação,
 * deixa a pessoa fazer ali mesmo. Enquanto está à vista, ele cuida do convite para ativar os avisos;
 * depois de terminado ou pulado, o convite avulso volta.
 */
export function TourSection({ me, hasPartner, composing }: Props) {
  const meId = me.id
  const { t } = useTranslation('onboarding')
  const { t: tp } = useTranslation('push')
  const { t: tc } = useTranslation('common')
  const reminders = useReminders(true)
  const { status, enable } = usePush()
  const [saved, setSaved] = useState(() => readSaved(meId))
  const [plan, setPlan] = useState<TourStepId[] | null>(null)
  const [index, setIndex] = useState(0)
  const [cheer, setCheer] = useState(0)
  const [generated, setGenerated] = useState(false)

  // "pronto" é ter respondido, com sucesso ou não: uma consulta que falha não pode esconder o convite para sempre
  const ready = !reminders.isPending && status !== undefined
  const hasReminder = (reminders.data?.length ?? 0) > 0
  const running = ready && !saved.dismissed

  // o plano é montado uma vez, quando o tour começa: um passo não some do meio só porque foi cumprido
  useEffect(() => {
    if (running && plan === null) setPlan(planTour({ hasPartner, hasReminder, cityChecked: saved.cityChecked === true, push: status }))
  }, [running, plan, hasPartner, hasReminder, saved.cityChecked, status])

  // "Ver o tour de novo" no menu da conta, com a pessoa já em Hoje
  useEffect(() => {
    const again = () => {
      setSaved(readSaved(meId))
      setPlan(null)
      setIndex(0)
      setGenerated(false)
    }
    window.addEventListener(RESTART_EVENT, again)
    return () => window.removeEventListener(RESTART_EVENT, again)
  }, [meId])

  const current = plan?.[Math.min(index, plan.length - 1)]
  const pushOn = status === 'on'
  const doneNow: Partial<Record<TourStepId, boolean>> = {
    reminder: hasReminder,
    city: saved.cityChecked === true,
    invite: hasPartner || generated,
    push: pushOn,
  }

  // a logo comemora quando o passo em que a pessoa está passa de pendente a feito
  const wasDone = useRef<string>('')
  const doneKey = `${current}:${current ? doneNow[current] === true : false}`
  useEffect(() => {
    const [id, done] = doneKey.split(':')
    const [prevId, prevDone] = wasDone.current.split(':')
    if (id === prevId && prevDone === 'false' && done === 'true') setCheer((c) => c + 1)
    wasDone.current = doneKey
  }, [doneKey])

  // criar o lembrete acontece no compositor, fora do tour: ao fechar, ele segue para o próximo passo.
  // Só vale o lembrete criado agora (de ausente a presente): quem volta a este passo depois não é empurrado adiante.
  const [advance, setAdvance] = useState(false)
  const hadReminder = useRef(hasReminder)
  useEffect(() => {
    if (!hadReminder.current && hasReminder && current === 'reminder') setAdvance(true)
    hadReminder.current = hasReminder
  }, [hasReminder, current])
  useEffect(() => {
    if (!advance || composing) return
    const id = window.setTimeout(() => {
      setAdvance(false)
      setIndex((i) => i + 1)
    }, 500)
    return () => window.clearTimeout(id)
  }, [advance, composing])
  useEffect(() => setAdvance(false), [current]) // saiu do passo por conta própria: nada a adiantar

  const pushError = !enable.isError
    ? null
    : enable.error instanceof PushError
      ? tp(`error.${enable.error.reason}`)
      : errorMessage(tc, enable.error)

  const steps: TourStep[] = (plan ?? []).map((id) => {
        const base = { id, target: TARGETS[id], title: t(`steps.${id}.title`), body: t(`steps.${id}.body`), done: doneNow[id] }
        switch (id) {
          case 'reminder':
            return { ...base, clickThrough: true, hint: t('steps.reminder.hint') }
          case 'city':
            return {
              ...base,
              content: (
                <CityPanel
                  timezone={me.timezone}
                  done={saved.cityChecked === true}
                  onChecked={() => {
                    writeSaved(meId, { cityChecked: true })
                    setSaved((s) => ({ ...s, cityChecked: true }))
                  }}
                />
              ),
            }
          case 'invite':
            return {
              ...base,
              content: hasPartner ? undefined : <InvitePanel onGenerated={() => setGenerated(true)} />,
              hint: undefined,
            }
          case 'push':
            return {
              ...base,
              body: status === 'install' ? t('steps.push.bodyInstall') : base.body,
              content: (
                <PushPanel
                  done={pushOn}
                  needsInstall={status === 'install'}
                  pending={enable.isPending}
                  error={pushError}
                  onEnable={() => enable.mutate()}
                />
              ),
            }
          default:
            return base
        }
  })

  function close() {
    writeSaved(meId, { dismissed: true })
    setSaved((s) => ({ ...s, dismissed: true }))
  }

  if (running && plan !== null) {
    return (
      <Tour
        steps={steps}
        index={index}
        onIndexChange={setIndex}
        onFinish={close}
        onClose={close}
        paused={composing}
        cheer={cheer}
        labels={{
          step: (n, total) => t('tour.step', { n, total }),
          next: t('tour.next'),
          back: t('tour.back'),
          skipStep: t('tour.skipStep'),
          skipTour: t('tour.skipTour'),
          finish: t('tour.finish'),
        }}
      />
    )
  }
  // tour terminado ou pulado: o convite para ativar os avisos aparece como antes; durante o tour o passo dos avisos cuida disso
  return <PushPrompt ready={ready && saved.dismissed === true} />
}
