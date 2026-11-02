import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Goal, Step } from './api'
import { GoalPage } from './GoalPage'
import { ctx, fakeServer, goal, renderWithQuery } from './testkit'

afterEach(() => vi.unstubAllGlobals())

const page = (id = 'g1') => renderWithQuery(<GoalPage id={id} ctx={ctx} />)
const emptyHistory = { from: '2026-08-01', to: '2026-11-02', days: [], periods: [] }

const shared = (over: Partial<Goal> = {}) =>
  goal({
    kind: 'shared', scope: 'household', owner_id: null, visibility: 'household', habit: undefined,
    shared: { target_count: 4, period: 'month', requires_both: true },
    progress: { done: 1, total: 4, percent: 25, unit: 'count', marked_today: false, period: { start: '2026-11-01', end: '2026-11-30' } },
    ...over,
  })

const financial = (over: Partial<Goal> = {}) =>
  goal({
    kind: 'financial', habit: undefined,
    financial: { target_cents: 100000, source: 'contributions', baseline_on: '2026-11-01' },
    progress: { done: 40000, total: 100000, percent: 40, unit: 'cents' },
    ...over,
  })

const step = (over: Partial<Step> = {}): Step => ({ id: 's1', position: 1, title: 'Pintar', assignee_id: null, done: false, has_task: false, ...over })
const project = (steps: Step[], over: Partial<Goal> = {}) =>
  goal({
    kind: 'project', scope: 'household', owner_id: null, visibility: 'household', habit: undefined, steps,
    progress: { done: steps.filter((s) => s.done).length, total: steps.length, percent: 0, unit: 'steps' }, ...over,
  })

describe('hábito', () => {
  it('marcar hoje: um clique, sem corpo, e o botão passa a anunciar o estado com Desfazer', async () => {
    let marked = false
    const server = fakeServer({
      'GET /v1/goals/g1': () => goal({ progress: { done: marked ? 2 : 1, total: 3, percent: 66, unit: 'count', marked_today: marked } }),
      'GET /v1/goals/g1/history': { ...emptyHistory, periods: [{ start: '2026-11-02', end: '2026-11-08', count: 1, target: 3 }] },
      'POST /v1/goals/g1/checkins': () => {
        marked = true
        return { created: true, goal: goal() }
      },
    })
    page()
    const button = await screen.findByRole('button', { name: 'Marcar hoje' })
    expect(button).toHaveAttribute('aria-pressed', 'false')
    expect(await screen.findByText('1 de 3')).toBeInTheDocument()

    await userEvent.click(button)
    expect(await screen.findByRole('button', { name: 'Marcado hoje' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Desfazer' })).toBeInTheDocument()
    expect(server.sent('POST', '/v1/goals/g1/checkins')[0].body).toBeUndefined()
    expect(await screen.findByText('2 de 3')).toBeInTheDocument()
  })

  it('histórico em fatos: período como número e dia fora dos combinados sem alarme', async () => {
    fakeServer({
      'GET /v1/goals/g1': goal({ habit: { target_count: 3, period: 'week', weekdays: 0b101, remind_min: 540, remind_enabled: false } }),
      'GET /v1/goals/g1/history': {
        ...emptyHistory,
        periods: [{ start: '2026-10-26', end: '2026-11-01', count: 0, target: 3 }],
        days: [{ day: '2026-11-03', by: ['me'], counted: false }],
      },
    })
    page()
    expect(await screen.findByText('0 de 3')).toBeInTheDocument()
    expect(await screen.findByText('fora dos dias combinados')).toBeInTheDocument()
    expect(screen.getByText(/Dias combinados: seg\.?, qua\.?/)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('checkin_date_invalid mostra a mensagem certa, sem reenviar', async () => {
    const server = fakeServer({
      'GET /v1/goals/g1': goal(),
      'GET /v1/goals/g1/history': emptyHistory,
      'POST /v1/goals/g1/checkins': { status: 400, code: 'checkin_date_invalid' },
    })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Marcar hoje' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Marque até hoje e no máximo 31 dias atrás.')
    expect(server.sent('POST', '/v1/goals/g1/checkins')).toHaveLength(1)
  })

  it('marcar outro dia oferece só os últimos 31 dias', async () => {
    fakeServer({ 'GET /v1/goals/g1': goal(), 'GET /v1/goals/g1/history': emptyHistory })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Marcar outro dia' }))
    const select = screen.getByLabelText('Dia (até 31 dias atrás)')
    expect(within(select).getAllByRole('option')).toHaveLength(32) // placeholder + 31 dias
    expect(screen.getByRole('button', { name: 'Marcar este dia' })).toBeDisabled()
  })
})

describe('privacidade', () => {
  it('meta do parceiro em view: só título, prazo e progresso; sem ações e sem pedir o histórico', async () => {
    const server = fakeServer({
      'GET /v1/goals/g1': goal({ owner_id: 'bia', created_by: 'bia', visibility: 'view', access: 'view', can_edit: false, notes: undefined }),
    })
    page()
    expect(await screen.findByText(/Bia compartilhou esta meta com você/)).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toBeInTheDocument()
    for (const name of ['Marcar hoje', 'Editar', 'Pausar', 'Concluir', 'Apagar']) {
      expect(screen.queryByRole('button', { name })).not.toBeInTheDocument()
    }
    expect(screen.queryByText('Histórico')).not.toBeInTheDocument()
    expect(server.calls.some((c) => c.url.includes('/history') || c.url.includes('/contributions'))).toBe(false)
  })

  it('404 diz "não encontramos", sem revelar que a meta existe nem que é privada', async () => {
    fakeServer({ 'GET /v1/goals/g1': { status: 404, code: 'goal_not_found' } })
    page()
    expect(await screen.findByText('Não encontramos essa meta')).toBeInTheDocument()
    expect(screen.queryByText(/privad|particular/i)).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Voltar às metas' })).toHaveAttribute('href', '/goals')
  })
})

describe('meta do casal', () => {
  it('requires_both com um só marcado: "Falta a outra pessoa", sem nome de culpado nem totais por pessoa', async () => {
    fakeServer({
      'GET /v1/goals/g1': shared({
        progress: { done: 0, total: 4, percent: 0, unit: 'count', marked_today: false, pending_days: ['2026-11-01'], period: { start: '2026-11-01', end: '2026-11-30' } },
        recent: { checkins: [{ day: '2026-11-01', by: ['bia'], counted: false }] },
      }),
      'GET /v1/goals/g1/history': emptyHistory,
    })
    page()
    expect(await screen.findByText(/Falta a outra pessoa em: 1 de nov/)).toBeInTheDocument()
    expect(screen.getByText('Conta quando os dois marcam.')).toBeInTheDocument()
    expect(screen.queryByText(/atrasad|devendo|ainda não marcou/i)).not.toBeInTheDocument()
  })

  it('mostra quem marcou hoje em ordem fixa por id', async () => {
    const today = new Intl.DateTimeFormat('en-CA', { timeZone: 'America/Sao_Paulo' }).format(new Date())
    fakeServer({
      'GET /v1/goals/g1': shared({
        progress: { done: 1, total: 4, percent: 25, unit: 'count', marked_today: true },
        recent: { checkins: [{ day: today, by: ['me', 'bia'], counted: true }] },
      }),
      'GET /v1/goals/g1/history': emptyHistory,
    })
    page()
    expect(await screen.findByText('Hoje marcou: Bia, Ana.')).toBeInTheDocument() // 'bia' < 'me'
  })
})

describe('meta financeira', () => {
  it('retirar manda o valor negativo em centavos; contribution_negative explica sem reenviar', async () => {
    const server = fakeServer({
      'GET /v1/goals/g1': financial(),
      'GET /v1/goals/g1/contributions': { items: [] },
      'POST /v1/goals/g1/contributions': { status: 409, code: 'contribution_negative' },
    })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Retirar' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/só um registro da meta. Não cria gasto/)).toBeInTheDocument()
    await userEvent.type(within(dialog).getByLabelText('Valor (R$)'), '1.234,56')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Retirar' }))

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('A retirada é maior que o valor guardado.')
    expect(server.sent('POST', '/v1/goals/g1/contributions')).toHaveLength(1)
    expect(server.sent('POST', '/v1/goals/g1/contributions')[0].body).toMatchObject({ amount_cents: -123456 })
  })

  it('ritmo em tom neutro e aporte lista quem registrou', async () => {
    fakeServer({
      'GET /v1/goals/g1': financial({
        due_on: '2027-03-01',
        progress: { done: 40000, total: 100000, percent: 40, unit: 'cents', on_track: false, required_per_month_cents: 33334 },
      }),
      'GET /v1/goals/g1/contributions': {
        items: [{ id: 'c1', amount_cents: 40000, made_on: '2026-11-01', created_by: 'me', created_by_name: 'Ana', created_at: '' }],
      },
    })
    page()
    expect(await screen.findByText(/Guardar R\$\s?333,34 por mês/)).toBeInTheDocument()
    expect(screen.getByText('Abaixo do ritmo')).toBeInTheDocument()
    expect(await screen.findByText('+R$ 400,00')).toBeInTheDocument()
    expect(screen.getByText(/Ana guardou/)).toBeInTheDocument()
  })

  it('economia do orçamento: sem botão de aporte; mês corrente provisório; Finanças desligado esconde o valor', async () => {
    const budget = (progress: Goal['progress']) =>
      financial({
        scope: 'household', owner_id: null, visibility: 'household',
        financial: { target_cents: 50000, source: 'budget_savings', category_id: 'c1', baseline_on: '2026-11-01' },
        progress,
      })
    fakeServer({ 'GET /v1/goals/g1': budget({ done: 12000, total: 50000, percent: 24, unit: 'cents', provisional: true }) })
    const first = page()
    expect(await screen.findByText('O mês atual ainda pode mudar.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Aportar' })).not.toBeInTheDocument()
    first.unmount()

    vi.unstubAllGlobals()
    fakeServer({ 'GET /v1/goals/g1': budget({ done: 0, total: 50000, percent: 0, unit: 'cents', unavailable: true }) })
    page()
    expect(await screen.findAllByText(/Disponível quando Finanças estiver ligado/)).not.toHaveLength(0)
    expect(screen.queryByText(/R\$\s?500,00/)).not.toBeInTheDocument()
  })
})

describe('projeto', () => {
  it('etapa de checklist conclui por /done; etapa com tarefa nunca tem checkbox próprio', async () => {
    const server = fakeServer({
      'GET /v1/goals/g1': project([
        step({ id: 's1', title: 'Pintar' }),
        step({ id: 's2', position: 2, title: 'Comprar tinta', has_task: true, task_id: 't1', task_status: 'open', assignee_id: 'bia' }),
        step({ id: 's3', position: 3, title: 'Medir', has_task: true, task_id: 't2', task_status: 'done', done: true, assignee_id: 'me' }),
        step({ id: 's4', position: 4, title: 'Contratar', has_task: true, task_id: 't3', task_status: 'declined' }),
      ]),
      'POST /v1/goals/g1/steps/s1/done': project([]),
    })
    page()
    const pintar = await screen.findByRole('checkbox', { name: 'Pintar' })
    expect(pintar).toBeEnabled()
    for (const name of ['Comprar tinta', 'Medir', 'Contratar']) expect(screen.getByRole('checkbox', { name })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'Medir' })).toBeChecked()
    expect(screen.getByText('Concluída na tarefa')).toBeInTheDocument()
    expect(screen.getByText('Recusada, sem responsável')).toBeInTheDocument()
    expect(screen.getByText('Conclui na tarefa')).toBeInTheDocument()

    await userEvent.click(pintar)
    await waitFor(() => expect(server.sent('POST', '/v1/goals/g1/steps/s1/done')).toHaveLength(1))
  })

  it('todas prontas: sugere concluir e não conclui sozinho', async () => {
    const server = fakeServer({
      'GET /v1/goals/g1': project([step({ done: true })], {
        progress: { done: 1, total: 1, percent: 100, unit: 'steps', suggest_complete: true },
      }),
    })
    page()
    expect(await screen.findByText('Todas as etapas estão prontas. Concluir a meta?')).toBeInTheDocument()
    expect(server.sent('POST', '/v1/goals/g1/complete')).toHaveLength(0)
  })

  it('sync_pending é um aviso discreto, não um erro', async () => {
    fakeServer({ 'GET /v1/goals/g1': project([step()], { sync_pending: true }) })
    page()
    expect(await screen.findByText(/Criando a tarefa/)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('reordenar manda a posição do vizinho', async () => {
    const server = fakeServer({
      'GET /v1/goals/g1': project([step({ id: 's1', position: 1, title: 'A' }), step({ id: 's2', position: 2, title: 'B' })]),
      'PUT /v1/goals/g1/steps/s2': project([]),
    })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Subir “B”' }))
    await waitFor(() => expect(server.sent('PUT', '/v1/goals/g1/steps/s2')).toHaveLength(1))
    expect(server.sent('PUT', '/v1/goals/g1/steps/s2')[0].body).toEqual({ position: 1 })
  })

  it('projeto pessoal: etapa sem "Virar tarefa" nem responsável', async () => {
    fakeServer({ 'GET /v1/goals/g1': project([step()], { scope: 'personal', owner_id: 'me', visibility: 'private' }) })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Editar “Pintar”' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).queryByRole('button', { name: 'Virar tarefa' })).not.toBeInTheDocument()
    expect(within(dialog).queryByLabelText('Responsável')).not.toBeInTheDocument()
  })
})

describe('estados e edição a dois', () => {
  it('concluir é decisão da pessoa: só chama /complete ao clicar; concluída oferece reabrir', async () => {
    let status: Goal['status'] = 'active'
    const server = fakeServer({
      'GET /v1/goals/g1': () => goal({ status, completed_by: status === 'done' ? 'me' : undefined, completed_at: status === 'done' ? '2026-11-02T12:00:00Z' : undefined }),
      'GET /v1/goals/g1/history': emptyHistory,
      'POST /v1/goals/g1/complete': () => {
        status = 'done'
        return goal({ status })
      },
    })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Concluir' }))
    await waitFor(() => expect(server.sent('POST', '/v1/goals/g1/complete')).toHaveLength(1))
    expect(await screen.findByText(/Concluída por Ana em/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reabrir' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Editar' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Marcar hoje' })).not.toBeInTheDocument()
  })

  it('goal_conflict recarrega, avisa e não reenvia sozinho', async () => {
    let title = 'Correr'
    const server = fakeServer({
      'GET /v1/goals/g1': () => goal({ title, version: title === 'Correr' ? 1 : 2 }),
      'GET /v1/goals/g1/history': emptyHistory,
      'PUT /v1/goals/g1': () => {
        title = 'Correr de manhã'
        return { status: 409, code: 'goal_conflict' }
      },
    })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Editar' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.type(within(dialog).getByLabelText('Título'), ' hoje')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Salvar' }))

    expect(await within(dialog).findByText(/Seu par mudou esta meta antes de você/)).toBeInTheDocument()
    expect(server.sent('PUT', '/v1/goals/g1')).toHaveLength(1)
    expect(server.sent('PUT', '/v1/goals/g1')[0].body).toMatchObject({ version: 1 })

    await userEvent.click(within(dialog).getByRole('button', { name: 'Ver a versão atual' }))
    expect(within(dialog).getByLabelText('Título')).toHaveValue('Correr de manhã')
    expect(server.sent('PUT', '/v1/goals/g1')).toHaveLength(1)
  })

  it('apagar pede confirmação e, em projeto do casal, avisa das tarefas', async () => {
    const server = fakeServer({
      'GET /v1/goals/g1': project([step()]),
      'DELETE /v1/goals/g1': null,
    })
    page()
    await userEvent.click(await screen.findByRole('button', { name: 'Apagar' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/As tarefas ligadas às etapas serão canceladas/)).toBeInTheDocument()
    expect(server.sent('DELETE', '/v1/goals/g1')).toHaveLength(0)
    await userEvent.click(within(dialog).getByRole('button', { name: 'Sim, apagar' }))
    await waitFor(() => expect(server.sent('DELETE', '/v1/goals/g1')).toHaveLength(1))
  })
})
