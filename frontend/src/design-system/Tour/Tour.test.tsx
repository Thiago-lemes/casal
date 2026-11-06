import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { Tour } from './Tour'
import type { TourLabels, TourStep } from './Tour'

const labels: TourLabels = {
  step: (n, total) => `Passo ${n} de ${total}`,
  next: 'Continuar',
  back: 'Voltar',
  skipStep: 'Pular',
  skipTour: 'Pular o tour',
  finish: 'Começar',
}

const steps: TourStep[] = [
  { id: 'a', target: 'alvo', title: 'Primeiro', body: 'Explica o primeiro.' },
  { id: 'b', title: 'Com ação', body: 'Faça agora.', content: <button type="button">Fazer</button> },
  { id: 'c', title: 'Fim', body: 'Acabou.' },
]

const noop = () => {}

function Harness({ list = steps, onFinish = noop, onClose = noop }: { list?: TourStep[]; onFinish?: () => void; onClose?: () => void }) {
  const [index, setIndex] = useState(0)
  return <Tour steps={list} index={index} onIndexChange={setIndex} onFinish={onFinish} onClose={onClose} labels={labels} />
}

function Fixed({ index, list = steps, paused = false, onFinish = noop }: { index: number; list?: TourStep[]; paused?: boolean; onFinish?: () => void }) {
  return <Tour steps={list} index={index} onIndexChange={noop} onFinish={onFinish} onClose={noop} labels={labels} paused={paused} />
}

/** jsdom não tem layout: dá um tamanho ao alvo para o tour o encontrar. */
function mountTarget() {
  const el = document.createElement('button')
  el.dataset.tour = 'alvo'
  el.textContent = 'Alvo'
  el.getBoundingClientRect = () => ({ top: 100, left: 20, width: 80, height: 40, right: 100, bottom: 140, x: 20, y: 100, toJSON: () => ({}) })
  document.body.appendChild(el)
  return el
}

const shields = () => document.body.querySelectorAll('[class*="shield"]').length

describe('Tour', () => {
  it('mostra o passo, explica, avança e volta', async () => {
    render(<Harness />)
    expect(screen.getByRole('dialog', { name: 'Primeiro' })).toBeInTheDocument()
    expect(screen.getByText('Passo 1 de 3')).toBeInTheDocument()
    expect(screen.getByText('Explica o primeiro.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Voltar' })).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Continuar' }))
    expect(screen.getByRole('dialog', { name: 'Com ação' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Voltar' }))
    expect(screen.getByRole('dialog', { name: 'Primeiro' })).toBeInTheDocument()
  })

  it('passo com ação oferece "Pular" até ser feito, e o conteúdo continua à vista', () => {
    const { rerender } = render(<Fixed index={1} />)
    expect(screen.getByRole('button', { name: 'Fazer' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Pular' })).toBeInTheDocument()

    rerender(<Fixed index={1} list={steps.map((s) => (s.id === 'b' ? { ...s, done: true } : s))} />)
    expect(screen.getByRole('button', { name: 'Continuar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Fazer' })).toBeInTheDocument()
  })

  it('o último passo termina em vez de continuar e não oferece pular', async () => {
    const onFinish = vi.fn()
    render(<Fixed index={2} onFinish={onFinish} />)
    expect(screen.queryByRole('button', { name: 'Pular o tour' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Começar' }))
    expect(onFinish).toHaveBeenCalledOnce()
  })

  it('Esc e "Pular o tour" fecham', async () => {
    const onClose = vi.fn()
    render(<Harness onClose={onClose} />)
    await userEvent.keyboard('{Escape}')
    await userEvent.click(screen.getByRole('button', { name: 'Pular o tour' }))
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('Esc dentro de um campo não fecha o tour (é do campo)', async () => {
    const onClose = vi.fn()
    const list: TourStep[] = [{ id: 'a', title: 'Cidade', body: 'x', content: <select aria-label="Cidade"><option>São Paulo</option></select> }, steps[2]]
    render(<Harness list={list} onClose={onClose} />)
    screen.getByRole('combobox', { name: 'Cidade' }).focus()
    await userEvent.keyboard('{Escape}')
    expect(onClose).not.toHaveBeenCalled()
    ;(document.activeElement as HTMLElement).blur()
    await userEvent.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('o foco vai para o título ao trocar de passo', async () => {
    render(<Harness />)
    expect(screen.getByRole('heading', { name: 'Primeiro' })).toHaveFocus()
    await userEvent.click(screen.getByRole('button', { name: 'Continuar' }))
    expect(screen.getByRole('heading', { name: 'Com ação' })).toHaveFocus()
  })

  it('pausado não aparece, mas não perde o lugar', () => {
    const { rerender } = render(<Fixed index={1} paused />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    rerender(<Fixed index={1} />)
    expect(screen.getByRole('dialog', { name: 'Com ação' })).toBeInTheDocument()
  })

  it('alvo ausente não quebra: o balão fica ao centro', () => {
    render(<Harness />)
    expect(screen.getByRole('dialog')).toHaveAttribute('data-side', 'center')
  })

  it('com alvo na tela, o balão aponta para ele e um escudo impede o toque', () => {
    const target = mountTarget()
    try {
      render(<Harness />)
      expect(screen.getByRole('dialog')).toHaveAttribute('data-side', 'below')
      expect(shields()).toBe(1)
    } finally {
      target.remove()
    }
  })

  it('passo com clickThrough deixa o alvo receber o toque (sem escudo)', () => {
    const target = mountTarget()
    try {
      render(<Fixed index={0} list={[{ ...steps[0], clickThrough: true }, steps[2]]} />)
      expect(shields()).toBe(0)
    } finally {
      target.remove()
    }
  })
})
