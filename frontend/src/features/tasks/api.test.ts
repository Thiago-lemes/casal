import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { tasksApi } from './api'

const fetchMock = vi.fn()

function reply(status: number, body?: unknown) {
  fetchMock.mockResolvedValue({ ok: status < 400, status, json: async () => body })
}

function lastCall() {
  const [url, init] = fetchMock.mock.calls.at(-1)!
  return { url: url as string, method: init.method as string, body: init.body ? JSON.parse(init.body) : undefined }
}

beforeEach(() => vi.stubGlobal('fetch', fetchMock))
afterEach(() => {
  vi.unstubAllGlobals()
  fetchMock.mockReset()
})

describe('tasksApi', () => {
  it('lista com um status por chamada', async () => {
    reply(200, [])
    await tasksApi.list('declined')
    expect(lastCall()).toMatchObject({ url: '/v1/tasks?status=declined', method: 'GET' })
  })

  it('trata lista nula como vazia', async () => {
    reply(200, null)
    await expect(tasksApi.list('open')).resolves.toEqual([])
  })

  it('cria com título, responsável, compartilhada e prazo', async () => {
    reply(201, {})
    await tasksApi.create({ title: 'Lavar o banheiro', assignee_id: '', shared: true, due_at: '2026-11-02T20:00:00.000Z' })
    expect(lastCall()).toEqual({
      url: '/v1/tasks',
      method: 'POST',
      body: { title: 'Lavar o banheiro', assignee_id: '', shared: true, due_at: '2026-11-02T20:00:00.000Z' },
    })
  })

  it('cria tarefa recorrente com rodízio', async () => {
    reply(201, {})
    await tasksApi.create({ title: 'Lixo', assignee_id: 'u1', shared: false, repeat: 'weekly', rotation: true })
    expect(lastCall().body).toEqual({ title: 'Lixo', assignee_id: 'u1', shared: false, repeat: 'weekly', rotation: true })
  })

  it('conclui, recusa, reatribui e cancela nas rotas certas', async () => {
    reply(200, {})
    await tasksApi.complete('t1')
    expect(lastCall()).toMatchObject({ url: '/v1/tasks/t1/complete', method: 'POST' })

    await tasksApi.decline('t1', 'Viajo nesse dia')
    expect(lastCall()).toEqual({ url: '/v1/tasks/t1/decline', method: 'POST', body: { reason: 'Viajo nesse dia' } })

    await tasksApi.assign('t1', { assignee_id: 'u2', shared: false })
    expect(lastCall()).toEqual({ url: '/v1/tasks/t1/assign', method: 'POST', body: { assignee_id: 'u2', shared: false } })

    reply(204)
    await tasksApi.cancel('t1')
    expect(lastCall()).toMatchObject({ url: '/v1/tasks/t1', method: 'DELETE' })
  })
})
