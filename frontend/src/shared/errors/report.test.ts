import { describe, expect, it } from 'vitest'
import { routeOf, scrub } from './report'

describe('relatório de erros', () => {
  it('não deixa passar conteúdo, URL com id, cookie nem e-mail', () => {
    const segredo = 'segredo-de-teste'
    const ev = scrub(
      {
        tags: {} as Record<string, string>,
        request: { url: `https://x/notes/1?q=${segredo}`, cookies: { a: segredo }, data: segredo },
        breadcrumbs: [{ message: segredo }],
        contexts: { x: segredo },
        extra: { x: segredo },
        message: segredo,
        user: { id: 'u1', email: `${segredo}@x.com` } as { id: string },
        exception: { values: [{ type: 'TypeError', value: `Cannot read ${segredo}` }] },
      },
      '/goals/0b9f4e1c-aaaa-bbbb-cccc-123456789012',
    )
    expect(JSON.stringify(ev)).not.toContain(segredo)
    expect(ev.user).toEqual({ id: 'u1' })
    expect(ev.exception?.values?.[0].type).toBe('TypeError')
    expect(ev.tags?.route).toBe('/goals/:id')
  })

  it('troca ids e números do caminho por marcadores', () => {
    expect(routeOf('/notes/42')).toBe('/notes/:id')
    expect(routeOf('/finance')).toBe('/finance')
  })
})
