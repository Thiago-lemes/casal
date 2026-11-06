/** Telas que o backend aceita no feedback (lista fechada, a mesma da tabela). */
export type FeedbackScreen =
  | 'today'
  | 'tasks'
  | 'notes'
  | 'routine'
  | 'shopping'
  | 'reminders'
  | 'house'
  | 'finance'
  | 'goals'
  | 'profile'
  | 'other'

/** Em que tela a pessoa estava quando abriu o feedback, a partir do caminho. Nunca envia o caminho em si (tem ids). */
export function screenFromPath(pathname: string): FeedbackScreen {
  const first = pathname.split('/').filter(Boolean)[0] ?? ''
  switch (first) {
    case '':
      return 'today'
    case 'tasks':
    case 'notes':
    case 'routine':
    case 'shopping':
    case 'reminders':
    case 'finance':
    case 'goals':
    case 'profile':
      return first
    case 'home':
      return 'house'
    default:
      return 'other'
  }
}
