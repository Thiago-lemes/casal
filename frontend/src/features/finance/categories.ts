import type { TFunction } from 'i18next'
import type { Category } from './api'

/** As categorias padrão têm `key` e nome traduzido; as criadas pelo casal mostram o `name`. */
export function categoryName(t: TFunction, c: Category | undefined): string {
  if (!c) return ''
  return c.key ? t(`category.${c.key}`, { ns: 'finance', defaultValue: c.name }) : c.name
}

/** Para um gasto novo, só as ativas; ao editar, a categoria atual fica visível mesmo se arquivada. */
export function selectable(all: Category[], keep?: string): Category[] {
  return all.filter((c) => !c.archived || c.id === keep)
}
