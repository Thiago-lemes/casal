import { describe, expect, it } from 'vitest'
import i18n from '@/shared/i18n'
import { ApiError } from './client'
import { errorMessage } from './errorMessage'

const t = i18n.t.bind(i18n)

describe('errorMessage', () => {
  it('traduz o code específico', () => {
    expect(errorMessage(t, new ApiError(404, 'task_not_found'))).toBe('Essa tarefa mudou ou não está mais disponível.')
  })

  it('sem texto para o code, cai no tipo geral do status', () => {
    expect(errorMessage(t, new ApiError(409, 'code_que_nao_existe'))).toBe('Isso já mudou. Atualize e tente de novo.')
  })

  it('nunca repete a frase do servidor', () => {
    expect(errorMessage(t, new ApiError(500, 'internal', 'erro interno'))).not.toContain('erro interno')
  })

  it('erro que não é da API vira a mensagem genérica', () => {
    expect(errorMessage(t, new Error('boom'))).toBe('Algo deu errado. Tente de novo.')
  })
})
