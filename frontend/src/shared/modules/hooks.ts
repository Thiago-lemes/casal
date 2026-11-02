import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/shared/api/client'
import { modulesApi } from './api'

export const modulesKey = ['modules'] as const

/** Os módulos opcionais do casal. Desligado é o padrão: módulo ausente da lista conta como desligado. */
export function useModules(enabled = true) {
  return useQuery({ queryKey: modulesKey, queryFn: modulesApi.list, enabled, staleTime: 60_000 })
}

export function useModuleEnabled(name: string, enabled = true) {
  const q = useModules(enabled)
  return { ...q, on: q.data?.some((m) => m.name === name && m.enabled) ?? false }
}

/** Liga ou desliga um módulo. Desligar esconde e não apaga: os dados voltam ao religar. */
export function useSetModule(name: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (on: boolean) => modulesApi.set(name, on),
    onSuccess: () => qc.invalidateQueries({ queryKey: modulesKey }),
  })
}

/** `403 module_disabled`: o módulo foi desligado. Trata-se como "desligado", não como falha. */
export function isModuleDisabled(err: unknown): boolean {
  return err instanceof ApiError && err.is('module_disabled')
}
