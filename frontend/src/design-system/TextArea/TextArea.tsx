import { useId } from 'react'
import type { TextareaHTMLAttributes } from 'react'
import styles from './TextArea.module.css'

type Props = Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'value'> & {
  label: string
  value: string
  maxLength: number
}

/** Texto longo com rótulo visível e contador de caracteres. */
export function TextArea({ label, value, maxLength, id, ...rest }: Props) {
  const auto = useId()
  const fieldId = id ?? auto
  return (
    <div className={styles.field}>
      <label htmlFor={fieldId} className={styles.label}>
        {label}
      </label>
      <textarea
        {...rest}
        id={fieldId}
        value={value}
        maxLength={maxLength}
        rows={rest.rows ?? 3}
        className={styles.input}
        aria-describedby={`${fieldId}-count`}
      />
      <p id={`${fieldId}-count`} className={styles.count}>
        {value.length}/{maxLength}
      </p>
    </div>
  )
}
