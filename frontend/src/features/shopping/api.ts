import { api } from '@/shared/api/client'

export type ShoppingCategory = 'produce' | 'grocery' | 'cleaning' | 'butcher' | 'other'

/** Na ordem em que a lista é mostrada (a do corredor do mercado: frescos primeiro). */
export const SHOPPING_CATEGORIES: ShoppingCategory[] = ['produce', 'butcher', 'grocery', 'cleaning', 'other']

export type Item = {
  id: string
  name: string
  quantity?: number | null
  unit: string
  category: ShoppingCategory
  /** volta sozinho à lista quando a pessoa pede para repor */
  recurring: boolean
  source: string
  added_by: string
  bought_at?: string | null
  bought_by?: string | null
  created_at: string
}

export type ItemInput = {
  name: string
  quantity: number | null
  unit: string
  category: ShoppingCategory
  recurring: boolean
}

/** Criar soma a quantidade num item aberto igual e responde `merged: true` (status 200 em vez de 201). */
export type CreateResult = Item & { merged: boolean }

export const shoppingApi = {
  /** Os abertos e os comprados nas últimas 24 horas (abertos primeiro). */
  list: async () => (await api<Item[] | null>('/v1/shopping/items')) ?? [],
  create: (v: ItemInput) => api<CreateResult>('/v1/shopping/items', { method: 'POST', body: v }),
  update: (id: string, v: ItemInput) => api<Item>(`/v1/shopping/items/${id}`, { method: 'PUT', body: v }),
  bought: (id: string) => api<Item>(`/v1/shopping/items/${id}/bought`, { method: 'POST' }),
  unbought: (id: string) => api<Item>(`/v1/shopping/items/${id}/unbought`, { method: 'POST' }),
  remove: (id: string) => api<void>(`/v1/shopping/items/${id}`, { method: 'DELETE' }),
  /** Traz de volta os itens recorrentes já comprados. */
  restock: () => api<{ restocked: number }>('/v1/shopping/restock', { method: 'POST' }),
}
