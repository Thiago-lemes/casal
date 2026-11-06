import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/design-system'
import { ZONES, allZones, canonicalZone, deviceZone, findZone, listedZone, zoneLabel } from './timezones'
import styles from './ZonePicker.module.css'

const OTHER = '__other__'

type Props = {
  label: string
  /** o fuso IANA escolhido */
  value: string
  /** o fuso escolhido; em "Outro fuso" pode ser o texto ainda inválido (quem salva confere com `isValidZone`) */
  onChange: (tz: string) => void
}

/** Escolha do fuso: lista curta com o deslocamento no rótulo, "Outro fuso" com busca e o atalho do aparelho. */
export function ZonePicker({ label, value, onChange }: Props) {
  const { t, i18n } = useTranslation('profile')
  const id = useId()
  const [other, setOther] = useState(false)
  const [draft, setDraft] = useState('')
  const zones = useMemo(() => (other ? allZones() : []), [other])
  const labels = useMemo(() => ZONES.map((zone) => ({ tz: zone.tz, text: zoneLabel(zone.tz, i18n.language) })), [i18n.language])
  const device = deviceZone()
  const known = findZone(value) !== undefined
  const draftInvalid = other && draft !== '' && canonicalZone(draft) === undefined

  function pick(v: string) {
    if (v === OTHER) {
      setOther(true)
      return
    }
    setOther(false)
    onChange(v)
  }

  function type(v: string) {
    setDraft(v)
    // o backend diferencia maiúsculas: vai o nome do banco, não o que a pessoa digitou
    onChange(canonicalZone(v) ?? v)
  }

  return (
    <div className={styles.picker}>
      <div className={styles.field}>
        <label htmlFor={id} className={styles.label}>
          {label}
        </label>
        <select id={id} value={other ? OTHER : listedZone(value)} onChange={(e) => pick(e.target.value)} className={styles.control}>
          {/* um fuso fora da lista (ex.: escolhido antes) continua selecionável */}
          {!known && !other && <option value={value}>{zoneLabel(value, i18n.language)}</option>}
          {labels.map((zone) => (
            <option key={zone.tz} value={zone.tz}>
              {zone.text}
            </option>
          ))}
          <option value={OTHER}>{t('zone.other')}</option>
        </select>
      </div>

      {other && (
        <div className={styles.field}>
          <label htmlFor={`${id}-other`} className={styles.label}>
            {t('zone.otherLabel')}
          </label>
          <input
            id={`${id}-other`}
            list={`${id}-list`}
            value={draft}
            onChange={(e) => type(e.target.value)}
            placeholder="America/Lima"
            autoCapitalize="off"
            autoCorrect="off"
            spellCheck={false}
            aria-invalid={draftInvalid}
            className={styles.control}
          />
          <datalist id={`${id}-list`}>
            {zones.map((tz) => (
              <option key={tz} value={tz} />
            ))}
          </datalist>
          {draftInvalid && (
            <p className={styles.hint} role="alert">
              {t('zone.invalid')}
            </p>
          )}
        </div>
      )}

      {device && listedZone(device) !== listedZone(value) && canonicalZone(device) && (
        <Button
          variant="quiet"
          onClick={() => {
            setOther(false)
            onChange(device)
          }}
        >
          {t('zone.useDevice', { zone: zoneLabel(device, i18n.language) })}
        </Button>
      )}
    </div>
  )
}
