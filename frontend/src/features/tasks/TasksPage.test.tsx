import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Task } from './api'
import { TasksPage } from './TasksPage'

const me = { id: 'me', name: 'Ana', timezone: 'America/Sao_Paulo' }
const people = [me, { id: 'other', name: 'Bia' }]

const task = (over: Partial<Task>): Task => ({
  id: 't1',
  created_by: 'me',
  created_by_name: 'Ana',
  assignee_id: 'me',
  title: 'Lavar o banheiro',
  status: 'open',
  created_at: '2026-11-01T10:00:00Z',
  ...over,
})

/** Servidor falso: guarda as tarefas e responde às rotas que a página usa. */
function fakeServer(initial: Task[]) {
  let tasks = [...initial]
  const calls: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init: { method?: string } = {}) => {
      const method = init.method ?? 'GET'
      calls.push(`${method} ${url}`)
      const ok = (body: unknown) => ({ ok: true, status: 200, json: async () => body })

      if (method === 'GET') {
        const status = new URL(url, 'http://x').searchParams.get('status')
        return ok(tasks.filter((t) => t.status === status))
      }
      const id = url.split('/')[3]
      if (url.endsWith('/complete')) {
        tasks = tasks.map((t) => (t.id === id ? { ...t, status: 'done' as const } : t))
        return ok(tasks.find((t) => t.id === id))
      }
      return ok({})
    }),
  )
  return { calls }
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <TasksPage me={me} people={people} />
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('TasksPage', () => {
  it('sem tarefas abertas: estado vazio que ensina o próximo passo', async () => {
    fakeServer([])
    renderPage()
    expect(await screen.findByText('Nada para fazer')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Nova tarefa' }).length).toBeGreaterThan(0)
  })

  it('cada aba tem o seu vazio e só busca ao abrir', async () => {
    const { calls } = fakeServer([])
    renderPage()
    await screen.findByText('Nada para fazer')
    expect(calls).toEqual(['GET /v1/tasks?status=open'])

    await userEvent.click(screen.getByRole('tab', { name: 'Recusadas' }))
    expect(await screen.findByText('Nenhuma tarefa recusada')).toBeInTheDocument()
    expect(calls).toContain('GET /v1/tasks?status=declined')
    expect(calls).not.toContain('GET /v1/tasks?status=done')
  })

  it('concluir tira a linha de Abertas', async () => {
    fakeServer([task({})])
    renderPage()
    const row = (await screen.findByText('Lavar o banheiro')).closest('li')!
    await userEvent.click(within(row).getByRole('button', { name: /Concluir tarefa/ }))
    await waitFor(() => expect(screen.queryByText('Lavar o banheiro')).not.toBeInTheDocument())
    expect(await screen.findByText('Nada para fazer')).toBeInTheDocument()
  })

  it('as abas respondem ao teclado', async () => {
    fakeServer([])
    renderPage()
    await screen.findByText('Nada para fazer')
    screen.getByRole('tab', { name: 'Abertas' }).focus()
    await userEvent.keyboard('{ArrowRight}')
    expect(screen.getByRole('tab', { name: 'Recusadas' })).toHaveAttribute('aria-selected', 'true')
  })
})
