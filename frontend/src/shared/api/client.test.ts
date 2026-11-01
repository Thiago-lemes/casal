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

  it('deriva o código do status quando o servidor não envia um', async () => {
    vi.stubGlobal('fetch', reply(403, { error: 'você ainda não tem um casal' }))
    await expect(api('/v1/household')).rejects.toMatchObject({ status: 403, code: 'forbidden' })
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
