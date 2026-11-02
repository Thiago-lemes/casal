import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { shoppingApi } from './api'
import type { Item, ItemInput } from './api'

export const shoppingKey = ['shopping'] as const

/** A lista é uma só para o casal; atualiza a cada 30 s para ver o que a outra pessoa marca no mercado. */
export function useShopping() {
  return useQuery({ queryKey: shoppingKey, queryFn: shoppingApi.list, refetchInterval: 30_000 })
}

export function useShoppingActions() {
  const qc = useQueryClient()
  const refresh = () => qc.invalidateQueries({ queryKey: shoppingKey })

  /** Marcar e desmarcar são otimistas: no mercado, com a mão ocupada, o toque precisa responder na hora. */
  const toggle = (api: (id: string) => Promise<Item>, patch: (i: Item) => Item) => ({
    mutationFn: api,
    onMutate: async (id: string) => {
      await qc.cancelQueries({ queryKey: shoppingKey })
      const previous = qc.getQueryData<Item[]>(shoppingKey)
      qc.setQueryData<Item[]>(shoppingKey, (list) => list?.map((i) => (i.id === id ? patch(i) : i)))
      return { previous }
    },
    onError: (_e: unknown, _id: string, ctx?: { previous?: Item[] }) => qc.setQueryData(shoppingKey, ctx?.previous),
    onSettled: refresh,
  })

  return {
    refresh,
    create: useMutation({ mutationFn: (v: ItemInput) => shoppingApi.create(v), onSuccess: refresh }),
    update: useMutation({ mutationFn: (v: { id: string; body: ItemInput }) => shoppingApi.update(v.id, v.body), onSuccess: refresh }),
    bought: useMutation(toggle(shoppingApi.bought, (i) => ({ ...i, bought_at: new Date().toISOString() }))),
    unbought: useMutation(toggle(shoppingApi.unbought, (i) => ({ ...i, bought_at: null }))),
    remove: useMutation({ mutationFn: shoppingApi.remove, onSuccess: refresh }),
    restock: useMutation({ mutationFn: shoppingApi.restock, onSuccess: refresh }),
  }
}

export type ShoppingActionsApi = ReturnType<typeof useShoppingActions>
