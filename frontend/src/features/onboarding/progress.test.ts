import { beforeEach, describe, expect, it } from 'vitest'
import { allDone, buildSteps, readSaved, writeSaved } from './progress'
import type { StepInput } from './progress'

const base: StepInput = { hasPartner: false, hasReminder: false, push: 'off', hasModule: false, cityChecked: false }
const ids = (i: StepInput) => buildSteps(i).map((s) => s.id)
const doneIds = (i: StepInput) => buildSteps(i).filter((s) => s.done).map((s) => s.id)

describe('buildSteps', () => {
  it('começa com tudo por fazer, na ordem da conversa', () => {
    expect(ids(base)).toEqual(['city', 'reminder', 'invite', 'push', 'modules'])
    expect(doneIds(base)).toEqual([])
    expect(allDone(buildSteps(base))).toBe(false)
  })

  it('marca como feito o que o app já sabe', () => {
    const all = { hasPartner: true, hasReminder: true, push: 'on', hasModule: true, cityChecked: true } as const
    expect(doneIds({ ...base, ...all })).toEqual(['city', 'reminder', 'invite', 'push', 'modules'])
    expect(allDone(buildSteps({ ...base, ...all }))).toBe(true)
  })

  it('quem entrou por convite já tem a outra pessoa: o passo vem feito', () => {
    expect(doneIds({ ...base, hasPartner: true })).toEqual(['invite'])
  })

  it('um passo cumprido não volta a pendente, mesmo que o estado de agora diga o contrário', () => {
    // apagou todos os lembretes; a outra pessoa saiu do casal
    const steps = buildSteps({ ...base, hasReminder: false, hasPartner: false, remembered: ['reminder', 'invite'] })
    expect(steps.filter((s) => s.done).map((s) => s.id)).toEqual(['reminder', 'invite'])
  })

  it('no iPhone fora da tela inicial o passo dos avisos continua pendente', () => {
    expect(buildSteps({ ...base, push: 'install' }).find((s) => s.id === 'push')).toEqual({ id: 'push', done: false })
  })

  it.each(['denied', 'unsupported', undefined] as const)('avisos "%s" não viram passo: não dá para cumprir', (push) => {
    expect(ids({ ...base, push })).toEqual(['city', 'reminder', 'invite', 'modules'])
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
})
