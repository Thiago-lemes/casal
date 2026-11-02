import { useEffect } from 'react'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { isModuleDisabled, modulesKey } from '@/shared/modules/hooks'
import { goalsApi } from './api'
import type { GoalInput, GoalQuery, GoalUpdate, StepInput, StepUpdate } from './api'

export const goalsKey = ['goals'] as const

export const keys = {
  list: (q: GoalQuery) => ['goals', 'list', q] as const,
  detail: (id: string) => ['goals', 'detail', id] as const,
  history: (id: string) => ['goals', 'history', id] as const,
  contributions: (id: string) => ['goals', 'contributions', id] as const,
}

/**
 * Toda ação muda o progresso, e o progresso é do servidor: recarrega o que a tela já abriu
 * em vez de recalcular no cliente. Sem atualização otimista de propósito.
 */
function useRefresh() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: goalsKey })
}

export function useGoals(query: GoalQuery, enabled = true) {
  return useQuery({ queryKey: keys.list(query), queryFn: () => goalsApi.list(query), enabled })
}

export function useGoal(id: string | undefined) {
  return useQuery({ queryKey: keys.detail(id ?? ''), queryFn: () => goalsApi.get(id!), enabled: Boolean(id) })
}

export function useGoalActions() {
  const refresh = useRefresh()
  return {
    create: useMutation({ mutationFn: (v: GoalInput) => goalsApi.create(v), onSuccess: refresh }),
    update: useMutation({ mutationFn: (v: { id: string; body: GoalUpdate }) => goalsApi.update(v.id, v.body), onSuccess: refresh }),
    remove: useMutation({ mutationFn: goalsApi.remove, onSuccess: refresh }),
    transition: useMutation({
      mutationFn: (v: { id: string; action: Parameters<typeof goalsApi.transition>[1] }) => goalsApi.transition(v.id, v.action),
      onSuccess: refresh,
    }),
    reload: refresh,
  }
}

/** Aportes, mais recente primeiro, com "Ver mais" pelo cursor `next_before`. */
export function useContributions(id: string, enabled = true) {
  return useInfiniteQuery({
    queryKey: keys.contributions(id),
    queryFn: ({ pageParam }) => goalsApi.contributions(id, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_before || undefined,
    enabled,
  })
}

export function useContributionActions(id: string) {
  const refresh = useRefresh()
  return {
    add: useMutation({ mutationFn: (v: { amount_cents: number; made_on?: string; note?: string }) => goalsApi.contribute(id, v), onSuccess: refresh }),
    remove: useMutation({ mutationFn: (cid: string) => goalsApi.removeContribution(id, cid), onSuccess: refresh }),
  }
}

/** Só quem tem `access = 'full'` pode pedir o histórico (o servidor responde 403 ao parceiro que só lê). */
export function useHistory(id: string, enabled = true) {
  return useQuery({ queryKey: keys.history(id), queryFn: () => goalsApi.history(id), enabled })
}

export function useCheckinActions(id: string) {
  const refresh = useRefresh()
  return {
    mark: useMutation({ mutationFn: (day?: string) => goalsApi.checkin(id, day), onSuccess: refresh }),
    undo: useMutation({ mutationFn: (day: string) => goalsApi.undoCheckin(id, day), onSuccess: refresh }),
  }
}

export function useStepActions(id: string) {
  const refresh = useRefresh()
  return {
    add: useMutation({ mutationFn: (v: StepInput) => goalsApi.addStep(id, v), onSuccess: refresh }),
    update: useMutation({ mutationFn: (v: { sid: string; body: StepUpdate }) => goalsApi.updateStep(id, v.sid, v.body), onSuccess: refresh }),
    remove: useMutation({ mutationFn: (sid: string) => goalsApi.removeStep(id, sid), onSuccess: refresh }),
    done: useMutation({ mutationFn: (v: { sid: string; done: boolean }) => goalsApi.stepDone(id, v.sid, v.done), onSuccess: refresh }),
    toTask: useMutation({ mutationFn: (sid: string) => goalsApi.stepToTask(id, sid), onSuccess: refresh }),
    dropTask: useMutation({ mutationFn: (sid: string) => goalsApi.stepDropTask(id, sid), onSuccess: refresh }),
  }
}

/** `403 module_disabled` não é falha: o módulo foi desligado. Atualiza os módulos e a tela mostra o convite. */
export function useModuleGuard(error: unknown) {
  const qc = useQueryClient()
  useEffect(() => {
    if (isModuleDisabled(error)) void qc.invalidateQueries({ queryKey: modulesKey })
  }, [error, qc])
}
