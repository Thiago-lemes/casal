import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/shared/api/client'
import { adminApi } from './api'

export const adminMeKey = ['admin', 'me'] as const

/** Quem está logado no painel. `null` = sem sessão (não é erro). */
export function useAdminMe() {
  return useQuery({
    queryKey: adminMeKey,
    queryFn: async () => {
      try {
        return await adminApi.me()
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) return null
        throw err
      }
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
}

export function useAdminLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { email: string; password: string; code: string }) => adminApi.login(v.email, v.password, v.code),
    onSuccess: (me) => qc.setQueryData(adminMeKey, me),
  })
}

export function useAdminLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: adminApi.logout,
    onSuccess: () => {
      qc.setQueryData(adminMeKey, null)
      qc.removeQueries({ predicate: (q) => q.queryKey[0] === 'admin' && q.queryKey[1] !== 'me' })
    },
  })
}

export function useSearch(q: string) {
  return useQuery({ queryKey: ['admin', 'search', q], queryFn: () => adminApi.search(q), enabled: q.length >= 2 })
}

export function useHouseholdFile(id: string) {
  // sem cache: abrir a ficha é uma leitura auditada, e cada abertura deve valer um registro
  return useQuery({ queryKey: ['admin', 'household', id], queryFn: () => adminApi.household(id), gcTime: 0, staleTime: 0 })
}

export function useSetModule(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { name: string; enabled: boolean }) => adminApi.setModule(id, v.name, v.enabled),
    onSuccess: ({ modules }) =>
      qc.setQueryData(['admin', 'household', id], (old: Awaited<ReturnType<typeof adminApi.household>> | undefined) =>
        old ? { ...old, modules } : old,
      ),
  })
}

export function useUserFile(id: string) {
  return useQuery({ queryKey: ['admin', 'user', id], queryFn: () => adminApi.user(id), gcTime: 0, staleTime: 0 })
}

export function useBlock(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (blocked: boolean) => adminApi.block(id, blocked),
    onSuccess: (user) => qc.setQueryData(['admin', 'user', id], user),
  })
}

export function useCodes() {
  return useQuery({ queryKey: ['admin', 'codes'], queryFn: adminApi.codes })
}

export function useCreateCode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { note: string; maxUses: number; days: number }) => adminApi.createCode(v.note, v.maxUses, v.days),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'codes'] }),
  })
}

export function useRevokeCode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: adminApi.revokeCode,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'codes'] }),
  })
}

export function useFeedbackList(unread: boolean) {
  return useQuery({ queryKey: ['admin', 'feedback', unread], queryFn: () => adminApi.feedback(unread) })
}

export function useReadFeedback() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: adminApi.readFeedback,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'feedback'] }),
  })
}

export function useAudit() {
  return useQuery({ queryKey: ['admin', 'audit'], queryFn: adminApi.audit, staleTime: 0 })
}
