import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/shared/api/client'
import { householdApi } from './api'

export const householdKey = ['household'] as const

/** O casal da pessoa. `data === null` significa "ainda sem casal" (403 da API). */
export function useHousehold() {
  return useQuery({
    queryKey: householdKey,
    queryFn: async () => {
      try {
        return await householdApi.get()
      } catch (err) {
        if (err instanceof ApiError && err.status === 403) return null
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
