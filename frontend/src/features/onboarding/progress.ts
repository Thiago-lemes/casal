export type TourStepId = 'nav' | 'reminder' | 'list' | 'city' | 'invite' | 'push' | 'end'

export type PlanInput = {
  /** a outra pessoa já entrou no casal */
  hasPartner: boolean
  /** existe algum lembrete no casal */
  hasReminder: boolean
  /** a pessoa já conferiu a cidade neste aparelho */
  cityChecked: boolean
  /** estado dos avisos neste aparelho (`undefined` enquanto não se sabe) */
  push: 'unsupported' | 'install' | 'denied' | 'off' | 'on' | undefined
}

/**
 * Os passos do tour para esta pessoa, na ordem da conversa. Só entra o que ainda faz sentido:
 * quem já tem lembrete, a outra pessoa no casal ou os avisos ligados não é mandado fazer de novo.
 * Avisos bloqueados ou sem suporte não viram passo (não dá para cumpri-lo). O plano é montado uma vez,
 * quando o tour começa: um passo não some do meio só porque a pessoa o cumpriu.
 */
export function planTour(i: PlanInput): TourStepId[] {
  const steps: TourStepId[] = ['nav']
  if (!i.hasReminder) steps.push('reminder')
  steps.push('list')
  if (!i.cityChecked) steps.push('city')
  if (!i.hasPartner) steps.push('invite')
  if (i.push === 'off' || i.push === 'install') steps.push('push')
  steps.push('end')
  return steps
}

/** O que fica guardado neste aparelho, por pessoa (duas contas no mesmo aparelho não se misturam). */
export type Saved = {
  cityChecked?: boolean
  /** o tour terminou ou foi pulado; "ver o tour de novo" volta a falso */
  dismissed?: boolean
}

const key = (userId: string) => `casal.onboarding.${userId}`

export function readSaved(userId: string): Saved {
  try {
    const raw = localStorage.getItem(key(userId))
    const data: unknown = raw ? JSON.parse(raw) : {}
    return typeof data === 'object' && data !== null ? (data as Saved) : {}
  } catch {
    return {}
  }
}

export function writeSaved(userId: string, patch: Saved) {
  try {
    localStorage.setItem(key(userId), JSON.stringify({ ...readSaved(userId), ...patch }))
  } catch {
    /* sem armazenamento: o tour volta a aparecer na próxima visita */
  }
}

export const RESTART_EVENT = 'casal:restart-tour'

/** "Ver o tour de novo": o tour reaparece em Hoje, já aberto se a pessoa estiver lá. */
export function restartTour(userId: string) {
  writeSaved(userId, { dismissed: false })
  window.dispatchEvent(new Event(RESTART_EVENT))
}
