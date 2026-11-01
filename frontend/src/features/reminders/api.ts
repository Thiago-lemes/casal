import { api } from '@/shared/api/client'

export type ReminderStatus = 'scheduled' | 'sent' | 'resolved' | 'canceled'

export type Reminder = {
  id: string
  created_by: string
  created_by_name: string
  recipient_id: string
  body: string
  remind_at: string
  status: ReminderStatus
  sent_at?: string
  resolved_at?: string
}

export const remindersApi = {
  list: () => api<Reminder[]>('/v1/reminders'),
  create: (v: { recipient_id: string; body: string; remind_at: string }) =>
    api<Reminder>('/v1/reminders', { method: 'POST', body: v }),
  resolve: (id: string) => api<Reminder>(`/v1/reminders/${id}/resolve`, { method: 'POST' }),
  snooze: (id: string, minutes: number) =>
    api<Reminder>(`/v1/reminders/${id}/snooze`, { method: 'POST', body: { minutes } }),
  cancel: (id: string) => api<void>(`/v1/reminders/${id}`, { method: 'DELETE' }),
}

/** Pendente = ainda precisa de uma ação do destinatário. */
export const isPending = (r: Reminder) => r.status === 'scheduled' || r.status === 'sent'
