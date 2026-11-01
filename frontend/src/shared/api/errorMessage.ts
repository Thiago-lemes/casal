import type { TFunction } from 'i18next'
import { ApiError } from './client'

/** Texto traduzido para um erro da API; nunca repete a frase do servidor. */
export function errorMessage(t: TFunction, err: unknown): string {
  const code = err instanceof ApiError ? err.code : 'unknown'
  return t(`errors.${code}`, { ns: 'common', defaultValue: t('errors.unknown', { ns: 'common' }) })
}
