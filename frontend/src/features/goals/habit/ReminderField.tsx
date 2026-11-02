import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { TextField } from '@/design-system'
import styles from '../goals.module.css'

/** Liga ou desliga o lembrete e escolhe o horário (no fuso de quem é o dono). */
export function ReminderField({ on, time, onChange }: { on: boolean; time: string; onChange: (v: { remindOn: boolean; remindTime: string }) => void }) {
  const { t } = useTranslation('goals')
  const id = useId()
  return (
    <div className={styles.form}>
      <label className={styles.check} htmlFor={id}>
        <input id={id} type="checkbox" checked={on} onChange={(e) => onChange({ remindOn: e.target.checked, remindTime: time })} />
        <span>
          {t('form.remind.label')}
          <small>{t('form.remind.hint')}</small>
        </span>
      </label>
      {on && <TextField label={t('form.remind.time')} type="time" value={time} onChange={(e) => onChange({ remindOn: on, remindTime: e.target.value })} required />}
    </div>
  )
}
