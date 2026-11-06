import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Mascot } from './Mascot'

describe('Mascot', () => {
  it('é decorativa: escondida de leitores de tela', () => {
    const { container } = render(<Mascot />)
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })

  it('leva o humor e o olhar como atributos', () => {
    const { container } = render(<Mascot mood="cheer" gaze={{ x: 1, y: -0.5 }} />)
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('data-mood', 'cheer')
    expect(svg.style.getPropertyValue('--gx')).toBe('1.8')
    expect(svg.style.getPropertyValue('--gy')).toBe('-0.8')
  })

  it('sozinha, entra cumprimentando', () => {
    const { container } = render(<Mascot playful />)
    expect(container.querySelector('svg')).toHaveAttribute('data-mood', 'hello')
  })
})
