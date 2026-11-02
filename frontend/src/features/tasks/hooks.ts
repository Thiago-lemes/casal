import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { tasksApi } from './api'
import type { ListStatus, Task } from './api'

export const tasksKey = ['tasks'] as const

/** Uma consulta por aba, para cada uma ter o seu cache; só busca quando a aba está aberta. */
export function useTasks(status: ListStatus, enabled = true) {
  return useQuery({
    queryKey: [...tasksKey, status],
    queryFn: () => tasksApi.list(status),
    enabled,
    // a aba Abertas é a que muda sozinha (o parceiro cria ou conclui)
    refetchInterval: status === 'open' ? 60_000 : false,
  })
}

/** Toda ação invalida `['tasks']` inteira, porque uma tarefa muda de aba. */
export function useTaskActions() {
  const qc = useQueryClient()
  const refresh = () => qc.invalidateQueries({ queryKey: tasksKey })
  const openKey = [...tasksKey, 'open']

  return {
    refresh,
    create: useMutation({ mutationFn: tasksApi.create, onSuccess: refresh }),

    // concluir é otimista: a linha sai de Abertas já, e volta se o servidor recusar
    complete: useMutation({
      mutationFn: tasksApi.complete,
      onMutate: async (id: string) => {
        await qc.cancelQueries({ queryKey: openKey })
        const previous = qc.getQueryData<Task[]>(openKey)
        qc.setQueryData<Task[]>(openKey, (list) => list?.filter((t) => t.id !== id))
        return { previous }
      },
      onError: (_err, _id, ctx) => qc.setQueryData(openKey, ctx?.previous),
      onSettled: refresh,
    }),

    // recusar, reatribuir e cancelar dependem de regra e justificativa: esperam o servidor
    decline: useMutation({
      mutationFn: (v: { id: string; reason: string }) => tasksApi.decline(v.id, v.reason),
      onSuccess: refresh,
    }),
    assign: useMutation({
      mutationFn: (v: { id: string; to: Parameters<typeof tasksApi.assign>[1] }) => tasksApi.assign(v.id, v.to),
      onSuccess: refresh,
    }),
    cancel: useMutation({ mutationFn: tasksApi.cancel, onSuccess: refresh }),
  }
}

export type TaskActionsApi = ReturnType<typeof useTaskActions>
