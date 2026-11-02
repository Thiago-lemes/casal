import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { barPercent, GoalProgress } from './GoalProgress'

describe('GoalProgress', () => {
  it('dinheiro: mostra o que o servidor calculou, em reais', () => {
    render(<GoalProgress label="Viagem" progress={{ done: 50000, total: 150000, percent: 33, unit: 'cents' }} />)
    const bar = screen.getByRole('progressbar', { name: 'Viagem' })
    expect(bar).toHaveAttribute('aria-valuenow', '33')
    expect(bar).toHaveAttribute('aria-valuetext', expect.stringMatching(/R\$\s?500,00 de R\$\s?1\.500,00/))
  })

  it('passa de 100 em hábito: a barra limita, o número fica', () => {
    render(<GoalProgress label="Correr" progress={{ done: 5, total: 3, percent: 166, unit: 'count' }} />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100')
    expect(screen.getByText('166%')).toBeInTheDocument()
    expect(screen.getByText('5 de 3')).toBeInTheDocument()
  })

  it('etapas', () => {
    render(<GoalProgress label="Reforma" progress={{ done: 2, total: 5, percent: 40, unit: 'steps' }} />)
    expect(screen.getByText('2 de 5 etapas')).toBeInTheDocument()
  })

  it('economia do orçamento indisponível: esconde o valor e explica', () => {
    render(<GoalProgress label="Mercado" progress={{ done: 0, total: 100000, percent: 0, unit: 'cents', unavailable: true }} />)
    expect(screen.getByText('Disponível quando Finanças estiver ligado')).toBeInTheDocument()
    expect(screen.queryByText(/R\$/)).not.toBeInTheDocument()
    expect(screen.getByRole('progressbar')).not.toHaveAttribute('aria-valuenow')
  })

  it('barPercent nunca sai de 0 a 100', () => {
    expect(barPercent({ done: 0, total: 1, percent: -5, unit: 'count' })).toBe(0)
    expect(barPercent({ done: 9, total: 1, percent: 900, unit: 'count' })).toBe(100)
  })
})
