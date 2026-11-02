import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, codeFromStatus } from './client'

const reply = (status: number, body?: unknown) =>
  vi.fn().mockResolvedValue({ ok: status < 400, status, json: async () => body })

afterEach(() => vi.unstubAllGlobals())

describe('api', () => {
  it('devolve o JSON em caso de sucesso', async () => {
    vi.stubGlobal('fetch', reply(200, { id: '1' }))
    await expect(api('/v1/me')).resolves.toEqual({ id: '1' })
  })

  it('aceita 204 sem corpo', async () => {
    vi.stubGlobal('fetch', reply(204))
    await expect(api('/x', { method: 'DELETE' })).resolves.toBeUndefined()
  })

  it('deriva o código do status quando o servidor não envia um (429)', async () => {
    vi.stubGlobal('fetch', reply(429, { error: 'muitas tentativas' }))
    await expect(api('/v1/auth/login')).rejects.toMatchObject({ status: 429, code: 'rate_limited' })
  })

  it('usa o code específico do servidor e guarda o tipo geral pelo status', async () => {
    vi.stubGlobal('fetch', reply(403, { code: 'no_household', error: 'você ainda não tem um casal' }))
    const err = (await api('/v1/household').catch((e) => e)) as ApiError
    expect(err.code).toBe('no_household')
    expect(err.kind).toBe('forbidden')
    expect(err.is('no_household')).toBe(true)
  })

  it('prefere o código enviado pelo servidor', async () => {
    vi.stubGlobal('fetch', reply(400, { code: 'reminder_in_past', error: 'x' }))
    await expect(api('/v1/reminders')).rejects.toMatchObject({ code: 'reminder_in_past' })
  })

  it('transforma falha de rede em ApiError network', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('offline')))
    const err = (await api("/x").catch((e) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe('network')
  })

  it('status desconhecido vira unknown', () => {
    expect(codeFromStatus(418)).toBe('unknown')
  })
})
