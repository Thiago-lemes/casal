import { beforeEach, describe, expect, it, vi } from 'vitest'
import { planTour, readSaved, restartTour, RESTART_EVENT, writeSaved } from './progress'
import type { PlanInput } from './progress'

const base: PlanInput = { hasPartner: false, hasReminder: false, cityChecked: false, push: 'off' }

describe('planTour', () => {
  it('quem está começando vê tudo, na ordem da conversa', () => {
    expect(planTour(base)).toEqual(['nav', 'reminder', 'list', 'city', 'invite', 'push', 'end'])
  })

  it('não manda refazer o que já está feito', () => {
    expect(planTour({ ...base, hasReminder: true })).not.toContain('reminder')
    expect(planTour({ ...base, cityChecked: true })).not.toContain('city')
    expect(planTour({ ...base, push: 'on' })).not.toContain('push')
  })

  it('quem entrou por convite já tem a outra pessoa: sem o passo do convite', () => {
    expect(planTour({ ...base, hasPartner: true })).toEqual(['nav', 'reminder', 'list', 'city', 'push', 'end'])
  })

  it('no iPhone fora da tela inicial o passo dos avisos continua (explica a instalação)', () => {
    expect(planTour({ ...base, push: 'install' })).toContain('push')
  })

  it.each(['denied', 'unsupported', undefined] as const)('avisos "%s" não viram passo: não dá para cumprir', (push) => {
    expect(planTour({ ...base, push })).not.toContain('push')
  })

  it('sempre abre com o menu e fecha com a despedida', () => {
    const all = planTour({ hasPartner: true, hasReminder: true, cityChecked: true, push: 'on' })
    expect(all).toEqual(['nav', 'list', 'end'])
  })
})

describe('estado guardado no aparelho', () => {
  beforeEach(() => localStorage.clear())

  it('guarda por pessoa e acumula os campos', () => {
    writeSaved('ana', { cityChecked: true })
    writeSaved('ana', { dismissed: true })
    expect(readSaved('ana')).toEqual({ cityChecked: true, dismissed: true })
    expect(readSaved('bia')).toEqual({})
  })

  it('ignora conteúdo corrompido', () => {
    localStorage.setItem('casal.onboarding.ana', '{não é json')
    expect(readSaved('ana')).toEqual({})
    localStorage.setItem('casal.onboarding.ana', '"texto"')
    expect(readSaved('ana')).toEqual({})
  })

  it('"ver o tour de novo" reabre o tour e avisa quem está ouvindo', () => {
    writeSaved('ana', { dismissed: true, cityChecked: true })
    const heard = vi.fn()
    window.addEventListener(RESTART_EVENT, heard)
    restartTour('ana')
    window.removeEventListener(RESTART_EVENT, heard)
    expect(readSaved('ana')).toEqual({ dismissed: false, cityChecked: true })
    expect(heard).toHaveBeenCalledOnce()
  })
})
