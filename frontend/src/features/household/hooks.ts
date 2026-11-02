import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/shared/api/client'
import { householdApi } from './api'

export const householdKey = ['household'] as const

/** O casal da pessoa. `data === null` significa "ainda sem casal" (`no_household` da API). */
export function useHousehold() {
  return useQuery({
    queryKey: householdKey,
    queryFn: async () => {
      try {
        return await householdApi.get()
      } catch (err) {
        if (err instanceof ApiError && err.is('no_household')) return null
        throw err
      }
    },
    retry: false,
  })
}

export function useCreateHousehold() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: householdApi.create,
    onSuccess: () => qc.invalidateQueries({ queryKey: householdKey }),
  })
}

export function useAcceptInvite() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: householdApi.accept,
    onSuccess: () => qc.invalidateQueries({ queryKey: householdKey }),
  })
}

export function useCreateInvite() {
  return useMutation({ mutationFn: householdApi.invite })
}

/** Sair do casal: depois disso as rotas do casal respondem `no_household` e o app volta à tela de montar ou entrar. */
export function useLeaveHousehold() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: householdApi.leave,
    onSuccess: () => {
      // o que estava em cache era do casal anterior
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'me' })
      qc.setQueryData(householdKey, null)
    },
  })
}
