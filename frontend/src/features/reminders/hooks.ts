import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { remindersApi } from './api'

export const remindersKey = ['reminders'] as const

/** Todos os lembretes (até 100, do mais recente ao mais antigo); a tela deriva o que precisa. */
export function useReminders(enabled: boolean) {
  return useQuery({
    queryKey: remindersKey,
    queryFn: remindersApi.list,
    enabled,
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
