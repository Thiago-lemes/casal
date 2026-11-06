import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/shared/api/client'
import { authApi } from './api'

export const meKey = ['me'] as const

/** Quem está logado. `data === null` significa "sem sessão" (não é erro). */
export function useMe() {
  return useQuery({
    queryKey: meKey,
    queryFn: async () => {
      try {
        return await authApi.me()
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) return null
        throw err
      }
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
}

/** Se o cadastro pede código de convite. Enquanto não responde (ou se falhar) assume aberto: o servidor decide de verdade. */
export function useSignupMode() {
  return useQuery({
    queryKey: ['auth-config'],
    queryFn: authApi.config,
    staleTime: 60 * 60 * 1000,
    retry: false,
  })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { email: string; password: string }) => authApi.login(v.email, v.password),
    onSuccess: (user) => qc.setQueryData(meKey, user),
  })
}

export function useRegister() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { email: string; name: string; password: string; signupCode?: string }) =>
      authApi.register(v.email, v.name, v.password, v.signupCode),
    onSuccess: (user) => qc.setQueryData(meKey, user),
  })
}

/** Muda o nome e/ou o fuso (campo ausente fica como está) e atualiza o perfil em cache. */
export function useUpdateProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: authApi.update,
    onSuccess: (user) => qc.setQueryData(meKey, user),
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: authApi.logout,
    onSuccess: () => {
      // grava "sem sessão" antes de limpar: um clear() remove a consulta que o RequireAuth observa,
      // e o redirecionamento para o login nunca acontece
      qc.setQueryData(meKey, null)
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== meKey[0] })
    },
  })
}
