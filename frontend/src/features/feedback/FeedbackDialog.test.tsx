import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FeedbackDialog } from './FeedbackDialog'
import { screenFromPath } from './screens'

function setup(path = '/tasks') {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <FeedbackDialog open onOpenChange={() => {}} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => vi.restoreAllMocks())

describe('FeedbackDialog', () => {
  it('avisa que a mensagem chega ao time e só habilita o envio com texto', async () => {
    setup()
    expect(screen.getByText(/chega para o time do Zoog/)).toBeInTheDocument()
    const send = screen.getByRole('button', { name: 'Enviar' })
    expect(send).toBeDisabled()
    await userEvent.type(screen.getByLabelText('Sua mensagem'), '   ')
    expect(send).toBeDisabled()
    await userEvent.type(screen.getByLabelText('Sua mensagem'), 'ajuda')
    expect(send).toBeEnabled()
  })

  it('envia o texto sem espaços nas pontas e a tela de onde veio, nunca o caminho', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)
    setup('/goals/0b9f4e1c-aaaa-bbbb-cccc-123456789012')
    await userEvent.type(screen.getByLabelText('Sua mensagem'), '  o botão sumiu  ')
    await userEvent.click(screen.getByRole('button', { name: 'Enviar' }))

    expect(await screen.findByText(/Recebemos/)).toBeInTheDocument()
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/v1/feedback')
    expect(JSON.parse(init.body as string)).toEqual({ message: 'o botão sumiu', screen: 'goals' })
  })

  it('mostra o erro traduzido quando o limite do dia estoura', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ code: 'feedback_rate_limited', error: 'x' }), { status: 409 })),
    )
    setup()
    await userEvent.type(screen.getByLabelText('Sua mensagem'), 'oi')
    await userEvent.click(screen.getByRole('button', { name: 'Enviar' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('muitas mensagens hoje')
  })
})

describe('screenFromPath', () => {
  it('traduz o caminho numa tela da lista fechada', () => {
    expect(screenFromPath('/')).toBe('today')
    expect(screenFromPath('/home')).toBe('house')
    expect(screenFromPath('/goals/abc')).toBe('goals')
    expect(screenFromPath('/qualquer-coisa')).toBe('other')
  })
})
