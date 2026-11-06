export type StepId = 'city' | 'reminder' | 'invite' | 'push' | 'modules'

export type Step = { id: StepId; done: boolean }

export type StepInput = {
  /** a outra pessoa já entrou no casal */
  hasPartner: boolean
  /** existe algum lembrete no casal */
  hasReminder: boolean
  /** estado dos avisos neste aparelho (`undefined` enquanto não se sabe) */
  push: 'unsupported' | 'install' | 'denied' | 'off' | 'on' | undefined
  /** Finanças ou Metas ligados */
  hasModule: boolean
  /** a pessoa já abriu o perfil para conferir a cidade */
  cityChecked: boolean
  /** passos que já foram vistos como feitos neste aparelho: o que se cumpriu não volta a pendente (ex.: apagou os lembretes) */
  remembered?: StepId[]
}

/**
 * Os primeiros passos e quais já foram dados, a partir do que o app já sabe.
 * Avisos bloqueados ou sem suporte não viram passo: não dá para cumpri-lo, então não cobramos.
 */
export function buildSteps(i: StepInput): Step[] {
  const steps: Step[] = [
    { id: 'city', done: i.cityChecked },
    { id: 'reminder', done: i.hasReminder },
    { id: 'invite', done: i.hasPartner },
  ]
  if (i.push === 'on' || i.push === 'off' || i.push === 'install') steps.push({ id: 'push', done: i.push === 'on' })
  steps.push({ id: 'modules', done: i.hasModule })
  return steps.map((s) => ({ ...s, done: s.done || (i.remembered?.includes(s.id) ?? false) }))
}

export const allDone = (steps: Step[]) => steps.every((s) => s.done)

/** O que fica guardado neste aparelho, por pessoa (duas contas no mesmo aparelho não se misturam). */
export type Saved = { cityChecked?: boolean; dismissed?: boolean; done?: StepId[] }

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
    /* sem armazenamento: o passo volta a aparecer na próxima visita */
  }
}
