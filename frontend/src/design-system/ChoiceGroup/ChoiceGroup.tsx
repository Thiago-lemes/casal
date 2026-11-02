import { useId } from 'react'
import styles from './ChoiceGroup.module.css'

type Props<V extends string> = {
  legend: string
  options: { value: V; label: string }[]
  value: V
  onChange: (value: V) => void
}

/** Uma escolha entre poucas opções, como botões de rádio do tamanho de um toque. */
export function ChoiceGroup<V extends string>({ legend, options, value, onChange }: Props<V>) {
  const name = useId()
  return (
    <fieldset className={styles.group}>
      <legend className={styles.legend}>{legend}</legend>
      <div className={styles.options}>
        {options.map((o) => (
          <label key={o.value} className={styles.option}>
            <input type="radio" name={name} value={o.value} checked={value === o.value} onChange={() => onChange(o.value)} />
            <span>{o.label}</span>
          </label>
        ))}
      </div>
    </fieldset>
  )
}
