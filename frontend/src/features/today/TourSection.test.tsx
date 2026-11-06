import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { readSaved, restartTour, writeSaved } from '@/features/onboarding/progress'
import { TourSection } from './TourSection'

vi.mock('@/features/push/hooks', () => ({
  PushError: class PushError extends Error {
    constructor(readonly reason: string) {
      super(reason)
    }
  },
  usePush: () => ({ status: 'off', enable: { mutate: vi.fn(), isPending: false, isError: false } }),
}))

const me = { id: 'ana', timezone: 'America/Sao_Paulo' }
let reminders: unknown[] = []

function setup(props: { hasPartner?: boolean; composing?: boolean } = {}) {
  vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, status: 200, json: async () => reminders })))
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const ui = (composing = false) => (
    <QueryClientProvider client={qc}>
      <TourSection me={me} hasPartner={props.hasPartner ?? false} composing={composing} />
    </QueryClientProvider>
  )
  const view = render(ui(props.composing))
  return { qc, view, ui }
}

beforeEach(() => {
  localStorage.clear()
  reminders = []
})
afterEach(() => vi.unstubAllGlobals())

describe('TourSection', () => {
  it('na primeira visita abre o tour pelo menu e conta os passos do plano', async () => {
    setup()
    expect(await screen.findByRole('dialog', { name: 'O mapa do app' })).toBeInTheDocument()
    expect(screen.getByText('Passo 1 de 7')).toBeInTheDocument()
  })

  it('quem já tem a outra pessoa e lembretes não vê esses passos', async () => {
    reminders = [{ id: 'r' }]
    setup({ hasPartner: true })
    expect(await screen.findByText('Passo 1 de 5')).toBeInTheDocument() // menu, lista, cidade, avisos, fim
  })

  it('pular o tour guarda no aparelho e faz o convite dos avisos voltar', async () => {
    setup()
    await screen.findByRole('dialog', { name: 'O mapa do app' })
    await userEvent.click(screen.getByRole('button', { name: 'Pular o tour' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(readSaved('ana').dismissed).toBe(true)
    expect(screen.getByText('Receba os avisos no celular')).toBeInTheDocument()
  })

  it('quem já pulou não vê o tour, e "ver de novo" o traz de volta', async () => {
    writeSaved('ana', { dismissed: true })
    setup()
    await screen.findByText('Receba os avisos no celular')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    act(() => restartTour('ana'))
    expect(await screen.findByRole('dialog', { name: 'O mapa do app' })).toBeInTheDocument()
    expect(readSaved('ana').dismissed).toBe(false)
  })

  it('durante o tour o convite avulso dos avisos espera', async () => {
    setup()
    await screen.findByRole('dialog', { name: 'O mapa do app' })
    expect(screen.queryByText('Receba os avisos no celular')).not.toBeInTheDocument()
  })

  it('enquanto o compositor está aberto o tour fica escondido', async () => {
    const { view, ui } = setup()
    await screen.findByRole('dialog', { name: 'O mapa do app' })
    view.rerender(ui(true))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    view.rerender(ui(false))
    expect(screen.getByRole('dialog', { name: 'O mapa do app' })).toBeInTheDocument()
  })

  it('criar o lembrete leva sozinho ao passo seguinte', async () => {
    const { qc } = setup()
    await screen.findByRole('dialog', { name: 'O mapa do app' })
    await userEvent.click(screen.getByRole('button', { name: 'Continuar' }))
    expect(screen.getByRole('dialog', { name: 'Crie o primeiro lembrete' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Pular' })).toBeInTheDocument()

    reminders = [{ id: 'novo' }]
    await act(() => qc.invalidateQueries({ queryKey: ['reminders'] }))
    expect(await screen.findByRole('dialog', { name: 'Seus lembretes ficam aqui' }, { timeout: 2000 })).toBeInTheDocument()
  })

  it('voltar ao passo do lembrete depois de criá-lo não empurra a pessoa adiante', async () => {
    const { qc } = setup()
    await screen.findByRole('dialog', { name: 'O mapa do app' })
    await userEvent.click(screen.getByRole('button', { name: 'Continuar' }))
    reminders = [{ id: 'novo' }]
    await act(() => qc.invalidateQueries({ queryKey: ['reminders'] }))
    await screen.findByRole('dialog', { name: 'Seus lembretes ficam aqui' }, { timeout: 2000 })

    await userEvent.click(screen.getByRole('button', { name: 'Voltar' }))
    expect(screen.getByRole('dialog', { name: 'Crie o primeiro lembrete' })).toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 900))
    expect(screen.getByRole('dialog', { name: 'Crie o primeiro lembrete' })).toBeInTheDocument()
  })

  it('o passo do fuso confirma com um toque e guarda', async () => {
    reminders = [{ id: 'r' }]
    setup({ hasPartner: true })
    await screen.findByText('Passo 1 de 5')
    await userEvent.click(screen.getByRole('button', { name: 'Continuar' })) // lista
    await userEvent.click(screen.getByRole('button', { name: 'Continuar' })) // cidade
    expect(screen.getByRole('dialog', { name: 'Seu fuso horário está certo?' })).toBeInTheDocument()
    expect(screen.getByText(/Brasília \(GMT−3\)/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Está certo' }))
    expect(screen.getByText('Fuso conferido.')).toBeInTheDocument()
    expect(readSaved('ana').cityChecked).toBe(true)
    expect(screen.getByRole('button', { name: 'Continuar' })).toBeInTheDocument()
  })

  it('terminar pelo último passo também encerra o tour', async () => {
    reminders = [{ id: 'r' }]
    writeSaved('ana', { cityChecked: true })
    setup({ hasPartner: true })
    await screen.findByText('Passo 1 de 4') // menu, lista, avisos, fim
    for (let i = 0; i < 3; i++) await userEvent.click(screen.getByRole('button', { name: i === 2 ? 'Pular' : 'Continuar' }))
    expect(screen.getByRole('dialog', { name: 'Tudo pronto!' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Começar a usar' }))
    expect(readSaved('ana').dismissed).toBe(true)
  })
})
