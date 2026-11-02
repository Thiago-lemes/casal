import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useLogout } from '@/features/auth/hooks'
import { RequireAuth } from './guards'

afterEach(() => vi.unstubAllGlobals())

const USER = { id: 'u1', email: 'ana@example.com', name: 'Ana', timezone: 'America/Sao_Paulo' }

function SignOut() {
  const logout = useLogout()
  return <button onClick={() => logout.mutate()}>Sair</button>
}

describe('RequireAuth', () => {
  it('ao sair, vai para o login sem precisar recarregar', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string, init: { method?: string } = {}) => {
        if (url === '/v1/me') return { ok: true, status: 200, json: async () => USER }
        if (url === '/v1/auth/logout' && init.method === 'POST') return { ok: true, status: 204, json: async () => null }
        return { ok: false, status: 404, json: async () => ({ code: 'not_found' }) }
      }),
    )
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter initialEntries={['/']}>
          <Routes>
            <Route path="/login" element={<p>Tela de login</p>} />
            <Route element={<RequireAuth />}>
              <Route index element={<SignOut />} />
            </Route>
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )

    await userEvent.click(await screen.findByRole('button', { name: 'Sair' }))
    expect(await screen.findByText('Tela de login')).toBeInTheDocument()
  })
})
