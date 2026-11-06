import { useCallback, useEffect, useState, useSyncExternalStore } from 'react'

export type ThemeChoice = 'system' | 'light' | 'dark'
export type PaletteChoice = 'blue' | 'orange'
const KEY = 'casal.theme'
const PALETTE_KEY = 'casal.palette'

function read(): ThemeChoice {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch {
    return 'system'
  }
}

let sessionPalette: PaletteChoice = 'blue' // vale quando não há armazenamento

function readPalette(): PaletteChoice {
  try {
    return localStorage.getItem(PALETTE_KEY) === 'orange' ? 'orange' : 'blue'
  } catch {
    return sessionPalette
  }
}

function applyPalette(p: PaletteChoice) {
  const root = document.documentElement
  if (p === 'orange') root.setAttribute('data-palette', 'orange')
  else root.removeAttribute('data-palette')
  applyBarColor()
}

/** A barra do navegador acompanha o tema e a paleta efetivos. */
function applyBarColor() {
  const root = document.documentElement
  const theme = root.getAttribute('data-theme')
  const dark = theme === 'dark' || (theme !== 'light' && matchMedia('(prefers-color-scheme: dark)').matches)
  const orange = root.getAttribute('data-palette') === 'orange'
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', dark ? '#1C1A19' : orange ? '#F26A1B' : '#E9EBEF')
}

function apply(choice: ThemeChoice) {
  const root = document.documentElement
  if (choice === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', choice)
  applyBarColor()
}

/** Aplica o tema salvo antes do primeiro desenho, para não piscar. */
export function initTheme() {
  apply(read())
  applyPalette(readPalette())
}

export function useTheme() {
  const [choice, setChoice] = useState<ThemeChoice>(read)

  useEffect(() => apply(choice), [choice])

  const update = useCallback((next: ThemeChoice) => {
    try {
      if (next === 'system') localStorage.removeItem(KEY)
      else localStorage.setItem(KEY, next)
    } catch {
      /* sem armazenamento: vale só nesta sessão */
    }
    setChoice(next)
  }, [])

  return [choice, update] as const
}

// A paleta é compartilhada: todas as instâncias do hook (menu do celular e do desktop) e as outras abas veem o mesmo valor.
const paletteListeners = new Set<() => void>()

function subscribePalette(listener: () => void) {
  paletteListeners.add(listener)
  const onStorage = (e: StorageEvent) => {
    if (e.key === PALETTE_KEY || e.key === null) {
      applyPalette(readPalette())
      listener()
    }
  }
  window.addEventListener('storage', onStorage)
  return () => {
    paletteListeners.delete(listener)
    window.removeEventListener('storage', onStorage)
  }
}

export function usePalette() {
  const palette = useSyncExternalStore(subscribePalette, readPalette, () => 'blue' as PaletteChoice)

  const update = useCallback((next: PaletteChoice) => {
    sessionPalette = next
    try {
      if (next === 'blue') localStorage.removeItem(PALETTE_KEY)
      else localStorage.setItem(PALETTE_KEY, next)
    } catch {
      /* sem armazenamento: vale só nesta sessão */
    }
    applyPalette(next)
    paletteListeners.forEach((l) => l())
  }, [])

  return [palette, update] as const
}
