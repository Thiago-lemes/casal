import type { TFunction } from 'i18next'
import { ApiError } from './client'

/**
 * Texto traduzido para um erro da API, pelo `code`; nunca repete a frase do servidor.
 * Sem texto para o `code` específico, cai no tipo geral do erro (pelo status).
 */
export function errorMessage(t: TFunction, err: unknown): string {
  if (!(err instanceof ApiError)) return t('errors.unknown', { ns: 'common' })
  return t(`errors.${err.code}`, {
    ns: 'common',
    defaultValue: t(`errors.${err.kind}`, { ns: 'common', defaultValue: t('errors.unknown', { ns: 'common' }) }),
  })
}
