import { render, screen } from '@testing-library/react'
import { Logo } from './Logo'

describe('Logo', () => {
  it('expõe o nome quando recebe title', () => {
    render(<Logo title="Zoog" />)
    expect(screen.getByRole('img', { name: 'Zoog' })).toBeInTheDocument()
  })

  it('fica escondida dos leitores de tela sem title', () => {
    const { container } = render(<Logo />)
    expect(screen.queryByRole('img')).toBeNull()
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })

  it('a versão só com o símbolo não desenha z nem g', () => {
    const { container } = render(<Logo variant="mark" />)
    expect(container.querySelector('polygon')).toBeNull()
  })

  it('duas logos na mesma tela não repetem ids de recorte', () => {
    const { container } = render(
      <>
        <Logo />
        <Logo />
      </>,
    )
    const ids = [...container.querySelectorAll('clipPath')].map((c) => c.id)
    expect(new Set(ids).size).toBe(ids.length)
  })
})
