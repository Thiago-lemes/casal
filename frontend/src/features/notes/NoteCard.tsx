import { Pin } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { VISIBILITY_ICONS } from '@/shared/ui/VisibilityPicker'
import type { Note } from './api'
import styles from './NoteCard.module.css'

const MAX_ITEMS = 4

type Props = {
  note: Note
  mine: boolean
  canEdit: boolean
  onOpen: () => void
  onTogglePin: () => void
  onToggleItem: (index: number) => void
}

/** Uma anotação: título, um pedaço do conteúdo e quem vê. A lista pode ser marcada direto no cartão. */
export function NoteCard({ note, mine, canEdit, onOpen, onTogglePin, onToggleItem }: Props) {
  const { t } = useTranslation('notes')
  const Icon = VISIBILITY_ICONS[note.visibility]
  const items = note.items ?? []
  const title = note.title || t('card.untitled')

  return (
    <li className={styles.card} data-pinned={note.pinned || undefined}>
      <div className={styles.head}>
        <button type="button" className={styles.open} onClick={onOpen}>
          {title}
        </button>
        <button
          type="button"
          className={styles.pin}
          aria-pressed={note.pinned}
          aria-label={note.pinned ? t('card.unpin', { title }) : t('card.pin', { title })}
          disabled={!canEdit}
          onClick={onTogglePin}
        >
          <Pin size={18} aria-hidden="true" />
        </button>
      </div>

      {note.kind === 'text' ? (
        note.body && <p className={styles.body}>{note.body}</p>
      ) : (
        <ul className={styles.items}>
          {items.slice(0, MAX_ITEMS).map((it, i) => (
            <li key={i}>
              <label className={styles.item} data-done={it.done || undefined}>
                <input type="checkbox" checked={it.done} disabled={!canEdit} onChange={() => onToggleItem(i)} />
                <span>{it.text}</span>
              </label>
            </li>
          ))}
          {items.length > MAX_ITEMS && <li className={styles.more}>{t('card.more', { count: items.length - MAX_ITEMS })}</li>}
        </ul>
      )}

      {(note.tags?.length ?? 0) > 0 && (
        <ul className={styles.tags} aria-label={t('card.tags')}>
          {note.tags!.map((tag) => (
            <li key={tag}>{tag}</li>
          ))}
        </ul>
      )}

      <p className={styles.meta}>
        <Icon size={16} aria-hidden="true" />
        <span>{t(`card.visibility.${note.visibility}`)}</span>
        {!mine && <span>{t('card.by', { name: note.owner_name })}</span>}
      </p>
    </li>
  )
}
