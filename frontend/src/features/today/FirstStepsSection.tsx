import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FirstSteps } from '@/features/onboarding/FirstSteps'
import { allDone, buildSteps, readSaved, writeSaved } from '@/features/onboarding/progress'
import { PushError, usePush } from '@/features/push/hooks'
import { PushPrompt } from '@/features/push/PushPrompt'
import { useReminders } from '@/features/reminders/hooks'
import { errorMessage } from '@/shared/api/errorMessage'
import { useModuleEnabled } from '@/shared/modules/hooks'

/**
 * Os primeiros passos no topo de Hoje. Enquanto a lista está à vista, ela cuida do convite para ativar os avisos.
 * Depois de dispensada ou concluída, o convite avulso volta; se a lista ainda estava em andamento, ele espera o primeiro lembrete.
 */
export function FirstStepsSection({ meId, hasPartner, onNewReminder }: { meId: string; hasPartner: boolean; onNewReminder: () => void }) {
  const { t: tp } = useTranslation('push')
  const { t: tc } = useTranslation('common')
  const reminders = useReminders(true)
  const finance = useModuleEnabled('finance')
  const goals = useModuleEnabled('goals')
  const { status, enable } = usePush()
  const [saved, setSaved] = useState(() => readSaved(meId))

  // "pronto" é ter respondido, com sucesso ou não: uma consulta que falha não pode esconder o convite para sempre.
  // O que veio com erro conta como "ainda não feito", e só o que veio com sucesso é guardado como feito.
  const ready = !reminders.isPending && !finance.isPending && !goals.isPending && status !== undefined
  const hasReminder = (reminders.data?.length ?? 0) > 0
  const steps = buildSteps({
    hasPartner,
    hasReminder,
    push: status,
    hasModule: finance.on || goals.on,
    cityChecked: saved.cityChecked === true,
    remembered: saved.done,
  })

  // um passo cumprido fica cumprido: apagar os lembretes ou o parceiro sair não traz a lista de volta
  const doneNow = steps.filter((s) => s.done).map((s) => s.id)
  const known = new Set(saved.done)
  const fresh = doneNow.filter((id) => !known.has(id))
  const freshKey = fresh.join(',')
  useEffect(() => {
    if (!ready || !freshKey) return
    const next = [...new Set([...(saved.done ?? []), ...(freshKey.split(',') as typeof fresh)])]
    writeSaved(meId, { done: next })
    setSaved((s) => ({ ...s, done: next }))
  }, [ready, freshKey, meId])

  if (ready && !saved.dismissed && !allDone(steps)) {
    const pushError = !enable.isError
      ? null
      : enable.error instanceof PushError
        ? tp(`error.${enable.error.reason}`)
        : errorMessage(tc, enable.error)
    return (
      <FirstSteps
        steps={steps}
        onNewReminder={onNewReminder}
        onEnablePush={() => enable.mutate()}
        pushPending={enable.isPending}
        pushError={pushError}
        pushNeedsInstall={status === 'install'}
        onDismiss={() => {
          writeSaved(meId, { dismissed: true })
          setSaved((s) => ({ ...s, dismissed: true }))
        }}
      />
    )
  }
  // lista dispensada: o convite aparece de qualquer jeito; lista em andamento (ou concluída): espera o primeiro lembrete
  return <PushPrompt ready={ready && (hasReminder || saved.dismissed === true)} />
}
