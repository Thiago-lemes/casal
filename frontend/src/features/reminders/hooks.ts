import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { isPending, remindersApi } from './api'

export const remindersKey = ['reminders'] as const

export function usePendingReminders(enabled: boolean) {
  return useQuery({
    queryKey: remindersKey,
    queryFn: remindersApi.list,
    enabled,
    select: (all) => all.filter(isPending).sort((a, b) => a.remind_at.localeCompare(b.remind_at)),
    refetchInterval: 60_000,
  })
}

/** Cada ação devolve a lista ao servidor; a tela decide quando refazer a consulta. */
export function useReminderActions() {
  const qc = useQueryClient()
  const refresh = () => qc.invalidateQueries({ queryKey: remindersKey })

  return {
    refresh,
    create: useMutation({ mutationFn: remindersApi.create, onSuccess: refresh }),
    resolve: useMutation({ mutationFn: remindersApi.resolve }),
    snooze: useMutation({
      mutationFn: (v: { id: string; minutes: number }) => remindersApi.snooze(v.id, v.minutes),
    }),
    cancel: useMutation({ mutationFn: remindersApi.cancel }),
  }
}
