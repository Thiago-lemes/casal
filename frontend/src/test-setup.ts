import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, beforeAll } from 'vitest'
import i18n from './shared/i18n'

// os testes procuram textos em português
beforeAll(async () => {
  await i18n.changeLanguage('pt-BR')
})

afterEach(() => cleanup())
