import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, Dialog, DialogClose, TextField } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import { VisibilityPicker } from '@/shared/ui/VisibilityPicker'
import { CATEGORIES } from './api'
import type { Block, BlockInput, Category } from './api'
import { CATEGORY_ICONS } from './categories'
import { draftFromBlock, emptyDraft, problems, toBody, toggleWeekday } from './form'
import type { Draft } from './form'
import styles from './BlockEditor.module.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** nulo = criar */
  block: Block | null
  /** hoje no fuso do perfil, para o padrão de um bloco novo */
  today: string
  isOwner: boolean
  readOnly: boolean
  pending?: boolean
  error?: unknown
  onSave: (body: BlockInput, version?: number) => void
  onDelete?: () => void
  onLoadCurrent?: () => Promise<Block>
}

/** Segunda a domingo, na ordem do backend (1 a 7); os rótulos vêm do idioma. */
const WEEK = [1, 2, 3, 4, 5, 6, 7]
const DAY_NAMES = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun']

/** Diálogo de criar, editar e ler um bloco da rotina. Monte com `key` para reiniciar o rascunho. */
export function BlockEditor({ open, onOpenChange, block, today, isOwner, readOnly, pending, error, onSave, onDelete, onLoadCurrent }: Props) {
  const { t } = useTranslation('routine')
  const { t: tc } = useTranslation('common')
  const [draft, setDraft] = useState<Draft>(() => (block ? draftFromBlock(block) : emptyDraft(today)))
  const [version, setVersion] = useState(block?.version)
  const [showProblems, setShowProblems] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [reloading, setReloading] = useState(false)

  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => setDraft((d) => ({ ...d, [key]: value }))
  const found = problems(draft)
  const conflict = error instanceof ApiError && error.is('routine_conflict')

  function submit(e: FormEvent) {
    e.preventDefault()
    if (readOnly) return
    if (found.length > 0) {
      setShowProblems(true)
      return
    }
    onSave(toBody(draft), version)
  }

  async function loadCurrent() {
    if (!onLoadCurrent) return
    setReloading(true)
    try {
      const fresh = await onLoadCurrent()
      setDraft(draftFromBlock(fresh))
      setVersion(fresh.version)
    } finally {
      setReloading(false)
    }
  }

  const err = (p: (typeof found)[number]) => (showProblems && found.includes(p) ? t(`editor.problem.${p}`) : undefined)

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={block ? (readOnly ? t('editor.read') : t('editor.edit')) : t('editor.new')}>
      <form className={styles.form} onSubmit={submit} noValidate>
        <TextField label={t('editor.title')} value={draft.title} onChange={(e) => set('title', e.target.value)} readOnly={readOnly} maxLength={120} error={err('title')} autoFocus={!block} />

        <fieldset className={styles.group} disabled={readOnly}>
          <legend className={styles.legend}>{t('editor.category')}</legend>
          <div className={styles.chips}>
            {CATEGORIES.map((c: Category) => {
              const Icon = CATEGORY_ICONS[c]
              return (
                <label key={c} className={styles.chip}>
                  <input type="radio" name="category" value={c} checked={draft.category === c} onChange={() => set('category', c)} />
                  <Icon size={16} aria-hidden="true" />
                  <span>{t(`category.${c}`)}</span>
                </label>
              )
            })}
          </div>
        </fieldset>

        <TextField label={t('editor.location')} hint={t('editor.locationHint')} value={draft.location} onChange={(e) => set('location', e.target.value)} readOnly={readOnly} maxLength={120} />

        <ChoiceGroup
          legend={t('editor.recurrence')}
          value={draft.recurrence}
          onChange={(v) => !readOnly && set('recurrence', v)}
          options={[
            { value: 'weekly', label: t('recurrence.weekly') },
            { value: 'once', label: t('recurrence.once') },
          ]}
        />

        {draft.recurrence === 'weekly' && (
          <>
            <fieldset className={styles.group} disabled={readOnly}>
              <legend className={styles.legend}>{t('editor.weekdays')}</legend>
              <div className={styles.chips}>
                {WEEK.map((d, i) => (
                  <label key={d} className={styles.day}>
                    <input type="checkbox" checked={draft.weekdays.includes(d)} onChange={() => set('weekdays', toggleWeekday(draft.weekdays, d))} />
                    <span>{t(`weekday.${DAY_NAMES[i]}`)}</span>
                  </label>
                ))}
              </div>
              {err('weekdays') && (
                <p className={styles.problem} role="alert">
                  {err('weekdays')}
                </p>
              )}
            </fieldset>

            <ChoiceGroup
              legend={t('editor.interval')}
              value={String(draft.intervalWeeks)}
              onChange={(v) => !readOnly && set('intervalWeeks', Number(v))}
              options={[1, 2, 3, 4].map((n) => ({ value: String(n), label: t(`interval.${n}`) }))}
            />
          </>
        )}

        <div className={styles.row}>
          <TextField label={t('editor.start')} type="time" value={draft.start} onChange={(e) => set('start', e.target.value)} readOnly={readOnly} />
          <TextField label={t('editor.end')} type="time" value={draft.end} onChange={(e) => set('end', e.target.value)} readOnly={readOnly} error={err('time')} />
        </div>

        <div className={styles.row}>
          <TextField
            label={draft.recurrence === 'once' ? t('editor.day') : t('editor.from')}
            type="date"
            value={draft.startDate}
            onChange={(e) => set('startDate', e.target.value)}
            readOnly={readOnly}
            error={err('dates')}
          />
          {draft.recurrence === 'weekly' && (
            <TextField label={t('editor.until')} hint={t('editor.untilHint')} type="date" value={draft.endDate} onChange={(e) => set('endDate', e.target.value)} readOnly={readOnly} />
          )}
        </div>

        <VisibilityPicker kind="routine" value={draft.visibility} onChange={(v) => set('visibility', v)} disabled={readOnly || !isOwner} />

        {error != null && (
          <div className={styles.error} role="alert">
            <p>{errorMessage(tc, error)}</p>
            {conflict && onLoadCurrent && (
              <Button loading={reloading} onClick={() => void loadCurrent()}>
                {t('editor.loadCurrent')}
              </Button>
            )}
          </div>
        )}

        <div className={styles.footer}>
          {block && isOwner && onDelete && (
            <Button variant="danger" className={styles.delete} onClick={() => (confirmDelete ? onDelete() : setConfirmDelete(true))}>
              {confirmDelete ? t('editor.confirmDelete') : t('editor.delete')}
            </Button>
          )}
          <DialogClose asChild>
            <Button variant="quiet">{readOnly ? tc('actions.close') : tc('actions.cancel')}</Button>
          </DialogClose>
          {!readOnly && (
            <Button type="submit" variant="primary" loading={pending}>
              {t('editor.save')}
            </Button>
          )}
        </div>
      </form>
    </Dialog>
  )
}
