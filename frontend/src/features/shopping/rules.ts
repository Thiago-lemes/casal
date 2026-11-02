import { SHOPPING_CATEGORIES } from './api'
import type { Item, ShoppingCategory } from './api'

export const isBought = (i: Item) => !!i.bought_at

/** Abertos agrupados por categoria, na ordem do mercado; categorias vazias não aparecem. */
export function groupOpen(items: Item[]): { category: ShoppingCategory; items: Item[] }[] {
  const open = items.filter((i) => !isBought(i))
  return SHOPPING_CATEGORIES.map((category) => ({
    category,
    items: open.filter((i) => i.category === category).sort((a, b) => a.name.localeCompare(b.name)),
  })).filter((g) => g.items.length > 0)
}

/** Comprados recentemente, o mais novo primeiro. */
export function boughtRecently(items: Item[]): Item[] {
  return items.filter(isBought).sort((a, b) => (b.bought_at ?? '').localeCompare(a.bought_at ?? ''))
}

/**
 * Lê uma quantidade digitada. Aceita vírgula ou ponto como decimal. Vazio é "sem quantidade" (null);
 * texto que não é número positivo devolve `undefined` (inválido).
 */
export function parseQuantity(text: string): number | null | undefined {
  const t = text.trim().replace(',', '.')
  if (t === '') return null
  if (!/^\d+(\.\d+)?$/.test(t)) return undefined
  const n = Number(t)
  return n > 0 ? n : undefined
}

/** "1,5 kg", "2 un", "3" ou "kg" (só unidade, sem quantidade, não aparece). */
export function formatQuantity(quantity: number | null | undefined, unit: string, lang: string): string {
  if (quantity === null || quantity === undefined) return ''
  const n = new Intl.NumberFormat(lang, { maximumFractionDigits: 3 }).format(quantity)
  return unit ? `${n} ${unit}` : n
}

/** Unidades sugeridas no campo (a pessoa pode digitar qualquer outra). */
export const UNIT_SUGGESTIONS = ['un', 'kg', 'g', 'L', 'ml', 'pct', 'cx', 'dz']
