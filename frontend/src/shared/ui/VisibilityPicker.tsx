import { Eye, EyeOff, Lock, Users } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import styles from './VisibilityPicker.module.css'

/** Quem vê (e quem edita) o que a pessoa cria. `busy` só existe na rotina: o parceiro vê só o horário. */
export type Visibility = 'private' | 'busy' | 'view' | 'household'

export const VISIBILITY_ICONS: Record<Visibility, LucideIcon> = {
  private: Lock,
  busy: EyeOff,
  view: Eye,
  household: Users,
}

type Props<V extends Visibility> = {
  /** `note` oferece privada, parceiro lê e do casal; `routine` acrescenta "só ocupado". */
  kind: 'note' | 'routine'
  value: V
  onChange: (value: V) => void
  /** O padrão deve ser o valor mais restrito (privado); quem usa o componente começa o estado nele. */
  disabled?: boolean
}

const OPTIONS: Record<'note' | 'routine', Visibility[]> = {
  note: ['private', 'view', 'household'],
  routine: ['private', 'busy', 'view', 'household'],
}

/** Um seletor único de visibilidade, reutilizado em anotações e rotina. */
export function VisibilityPicker<V extends Visibility>({ kind, value, onChange, disabled }: Props<V>) {
  const { t } = useTranslation()
  const name = useId()

  return (
    <fieldset className={styles.group} disabled={disabled}>
      <legend className={styles.legend}>{t('visibility.legend')}</legend>
      <div className={styles.options}>
        {OPTIONS[kind].map((v) => {
          const Icon = VISIBILITY_ICONS[v]
          return (
            <label key={v} className={styles.option}>
              <input type="radio" name={name} value={v} checked={value === v} onChange={() => onChange(v as V)} />
              <Icon size={20} aria-hidden="true" className={styles.icon} />
              <span>
                <span className={styles.title}>{t(`visibility.${kind}.${v}.title`)}</span>
                <span className={styles.hint}>{t(`visibility.${kind}.${v}.hint`)}</span>
              </span>
            </label>
          )
        })}
      </div>
    </fieldset>
  )
}
