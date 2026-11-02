import { Plus, X } from 'lucide-react'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, Dialog, DialogClose, TextArea, TextField } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import { VisibilityPicker } from '@/shared/ui/VisibilityPicker'
import type { Note, NoteInput, NoteItem, NoteKind, NoteVisibility } from './api'
import { cleanItems, parseTags } from './rules'
import styles from './NoteEditor.module.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** nulo = criar */
  note: Note | null
  /** quem pode mudar a visibilidade e apagar (só o dono); em anotação nova é verdadeiro */
  isOwner: boolean
  /** em anotação do parceiro só de leitura, nada é editável */
  readOnly: boolean
  pending?: boolean
  error?: unknown
  /** `version` só existe ao editar */
  onSave: (input: NoteInput, version?: number) => void
  onDelete?: () => void
  /** busca a versão atual da anotação, depois de um conflito */
  onLoadCurrent?: () => Promise<Note>
}

const MAX_BODY = 20000

/** Diálogo de criar, editar e ler uma anotação (texto ou lista). Monte com `key` para reiniciar o rascunho. */
export function NoteEditor({ open, onOpenChange, note, isOwner, readOnly, pending, error, onSave, onDelete, onLoadCurrent }: Props) {
  const { t } = useTranslation('notes')
  const { t: tc } = useTranslation('common')

  const [version, setVersion] = useState(note?.version)
  const [title, setTitle] = useState(note?.title ?? '')
  const [kind, setKind] = useState<NoteKind>(note?.kind ?? 'text')
  const [body, setBody] = useState(note?.body ?? '')
  const [items, setItems] = useState<NoteItem[]>(note?.items?.length ? note.items : [{ text: '', done: false }])
  const [tags, setTags] = useState((note?.tags ?? []).join(', '))
  // o padrão é o valor mais restrito
  const [visibility, setVisibility] = useState<NoteVisibility>(note?.visibility ?? 'private')
  const [pinned, setPinned] = useState(note?.pinned ?? false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [reloading, setReloading] = useState(false)

  const conflict = error instanceof ApiError && error.is('note_conflict')
  const cleaned = cleanItems(items)
  const empty = !title.trim() && (kind === 'text' ? !body.trim() : cleaned.length === 0)

  function submit(e: FormEvent) {
    e.preventDefault()
    if (readOnly || empty) return
    onSave(
      {
        title: title.trim(),
        kind,
        body: kind === 'text' ? body : '',
        items: kind === 'checklist' ? cleaned : [],
        tags: parseTags(tags),
        visibility,
        pinned,
      },
      version,
    )
  }

  async function loadCurrent() {
    if (!onLoadCurrent) return
    setReloading(true)
    try {
      const fresh = await onLoadCurrent()
      setVersion(fresh.version)
      setTitle(fresh.title)
      setBody(fresh.body)
      setItems(fresh.items?.length ? fresh.items : [{ text: '', done: false }])
      setTags((fresh.tags ?? []).join(', '))
      setVisibility(fresh.visibility)
      setPinned(fresh.pinned)
    } finally {
      setReloading(false)
    }
  }

  function setItem(i: number, patch: Partial<NoteItem>) {
    setItems((list) => list.map((it, j) => (j === i ? { ...it, ...patch } : it)))
  }

  const heading = note ? (readOnly ? t('editor.read') : t('editor.edit')) : t('editor.new')

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={heading}>
      <form className={styles.form} onSubmit={submit}>
        <TextField
          label={t('editor.title')}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          maxLength={200}
          readOnly={readOnly}
          autoFocus={!note}
        />

        {!note && (
          <ChoiceGroup
            legend={t('editor.kind')}
            value={kind}
            onChange={setKind}
            options={[
              { value: 'text', label: t('kind.text') },
              { value: 'checklist', label: t('kind.checklist') },
            ]}
          />
        )}

        {kind === 'text' ? (
          <TextArea label={t('editor.body')} value={body} maxLength={MAX_BODY} rows={6} readOnly={readOnly} onChange={(e) => setBody(e.target.value)} />
        ) : (
          <fieldset className={styles.items}>
            <legend className={styles.legend}>{t('editor.items')}</legend>
            {items.map((it, i) => (
              <div key={i} className={styles.item}>
                <input
                  type="checkbox"
                  checked={it.done}
                  disabled={readOnly}
                  onChange={(e) => setItem(i, { done: e.target.checked })}
                  aria-label={t('editor.itemDone', { n: i + 1 })}
                />
                <input
                  className={styles.itemText}
                  value={it.text}
                  readOnly={readOnly}
                  maxLength={500}
                  aria-label={t('editor.itemText', { n: i + 1 })}
                  onChange={(e) => setItem(i, { text: e.target.value })}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !readOnly) {
                      e.preventDefault()
                      if (i === items.length - 1) setItems((l) => [...l, { text: '', done: false }])
                    }
                  }}
                />
                {!readOnly && (
                  <button
                    type="button"
                    className={styles.remove}
                    onClick={() => setItems((l) => (l.length > 1 ? l.filter((_, j) => j !== i) : [{ text: '', done: false }]))}
                    aria-label={t('editor.removeItem', { n: i + 1 })}
                  >
                    <X size={18} aria-hidden="true" />
                  </button>
                )}
              </div>
            ))}
            {!readOnly && (
              <div>
                <Button variant="quiet" onClick={() => setItems((l) => [...l, { text: '', done: false }])}>
                  <Plus size={18} aria-hidden="true" />
                  {t('editor.addItem')}
                </Button>
              </div>
            )}
          </fieldset>
        )}

        <TextField label={t('editor.tags')} hint={t('editor.tagsHint')} value={tags} readOnly={readOnly} onChange={(e) => setTags(e.target.value)} />

        <VisibilityPicker kind="note" value={visibility} onChange={setVisibility} disabled={readOnly || !isOwner} />

        <label className={styles.check}>
          <input type="checkbox" checked={pinned} disabled={readOnly} onChange={(e) => setPinned(e.target.checked)} />
          <span>{t('editor.pin')}</span>
        </label>

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
          {note && isOwner && onDelete && (
            <Button variant="danger" className={styles.delete} onClick={() => (confirmDelete ? onDelete() : setConfirmDelete(true))}>
              {confirmDelete ? t('editor.confirmDelete') : t('editor.delete')}
            </Button>
          )}
          <DialogClose asChild>
            <Button variant="quiet">{readOnly ? tc('actions.close') : tc('actions.cancel')}</Button>
          </DialogClose>
          {!readOnly && (
            <Button type="submit" variant="primary" loading={pending} disabled={empty}>
              {t('editor.save')}
            </Button>
          )}
        </div>
      </form>
    </Dialog>
  )
}
