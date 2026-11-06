import { api } from '@/shared/api/client'
import type { FeedbackScreen } from './screens'

export const feedbackApi = {
  send: (message: string, screen: FeedbackScreen) => api<void>('/v1/feedback', { method: 'POST', body: { message, screen } }),
}
