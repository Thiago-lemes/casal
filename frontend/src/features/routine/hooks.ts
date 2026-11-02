import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { routineApi } from './api'
import type { BlockInput, ExceptionBody } from './api'

export const routineKey = ['routine'] as const

export function useBlocks() {
  return useQuery({ queryKey: [...routineKey, 'blocks'], queryFn: routineApi.blocks })
}

/** O que acontece, dos dois, entre dois dias. Ao navegar, mostra o intervalo anterior até o novo chegar. */
export function useOccurrences(from: string, to: string) {
  return useQuery({
    queryKey: [...routineKey, 'occurrences', from, to],
    queryFn: () => routineApi.occurrences(from, to),
    placeholderData: keepPreviousData,
  })
}

/** Janelas em que os dois estão livres no dia. */
export function useFree(date: string, min: number, enabled = true) {
  return useQuery({ queryKey: [...routineKey, 'free', date, min], queryFn: () => routineApi.free(date, min), enabled })
}

/** Quem está onde agora; atualiza a cada minuto. */
export function useNow() {
  return useQuery({ queryKey: [...routineKey, 'now'], queryFn: routineApi.now, refetchInterval: 60_000 })
}

/** Toda mudança na rotina invalida `['routine']` inteira: blocos, dias, horários livres e "agora". */
export function useRoutineActions() {
  const qc = useQueryClient()
  const refresh = () => qc.invalidateQueries({ queryKey: routineKey })

  return {
    refresh,
    create: useMutation({ mutationFn: (v: BlockInput) => routineApi.create(v), onSuccess: refresh }),
    update: useMutation({
      mutationFn: (v: { id: string; body: BlockInput & { version: number } }) => routineApi.update(v.id, v.body),
      onSuccess: refresh,
    }),
    remove: useMutation({ mutationFn: routineApi.remove, onSuccess: refresh }),
    setException: useMutation({
      mutationFn: (v: { id: string; date: string; body: ExceptionBody }) => routineApi.setException(v.id, v.date, v.body),
      onSuccess: refresh,
    }),
    clearException: useMutation({
      mutationFn: (v: { id: string; date: string }) => routineApi.clearException(v.id, v.date),
      onSuccess: refresh,
    }),
  }
}

export type RoutineActionsApi = ReturnType<typeof useRoutineActions>
