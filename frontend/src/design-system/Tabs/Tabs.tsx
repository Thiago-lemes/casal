import { useId, useRef } from 'react'
import type { KeyboardEvent, ReactNode } from 'react'
import styles from './Tabs.module.css'

type Props<V extends string> = {
  label: string
  tabs: { value: V; label: string }[]
  value: V
  onChange: (value: V) => void
  children: ReactNode
}

/** Abas acessíveis: setas, Home e End movem o foco e a seleção; o painel é o `children`. */
export function Tabs<V extends string>({ label, tabs, value, onChange, children }: Props<V>) {
  const id = useId()
  const refs = useRef<(HTMLButtonElement | null)[]>([])

  function onKeyDown(e: KeyboardEvent, index: number) {
    const last = tabs.length - 1
    const next =
      e.key === 'ArrowRight' ? (index === last ? 0 : index + 1)
      : e.key === 'ArrowLeft' ? (index === 0 ? last : index - 1)
      : e.key === 'Home' ? 0
      : e.key === 'End' ? last
      : null
    if (next === null) return
    e.preventDefault()
    onChange(tabs[next].value)
    refs.current[next]?.focus()
  }

  return (
    <div className={styles.root}>
      <div role="tablist" aria-label={label} className={styles.list}>
        {tabs.map((tab, i) => (
          <button
            key={tab.value}
            ref={(el) => {
              refs.current[i] = el
            }}
            type="button"
            role="tab"
            id={`${id}-tab-${tab.value}`}
            aria-selected={value === tab.value}
            aria-controls={`${id}-panel`}
            tabIndex={value === tab.value ? 0 : -1}
            className={styles.tab}
            onClick={() => onChange(tab.value)}
            onKeyDown={(e) => onKeyDown(e, i)}
          >
            {tab.label}
          </button>
        ))}
      </div>
      <div role="tabpanel" id={`${id}-panel`} aria-labelledby={`${id}-tab-${value}`} className={styles.panel}>
        {children}
      </div>
    </div>
  )
}
