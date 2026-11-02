import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, EmptyState, Skeleton, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { notesApi } from './api'
import type { Note, NoteInput } from './api'
import { useNoteActions, useNotes } from './hooks'
import { NoteCard } from './NoteCard'
import { NoteEditor } from './NoteEditor'
import { allTags, filterNotes, noteCan, sortNotes, withChange } from './rules'
import styles from './NotesPage.module.css'

/** `me` é quem está logado; a tela que compõe as features entrega isso. */
export function NotesPage({ me }: { me: { id: string } }) {
  const { t } = useTranslation('notes')
  const { t: tc } = useTranslation('common')
  const notes = useNotes()
  const actions = useNoteActions()
  const [query, setQuery] = useState('')
  const [tag, setTag] = useState<string | null>(null)
  const [editing, setEditing] = useState<Note | 'new' | null>(null)
  const [notice, setNotice] = useState<string>()

  const all = notes.data ?? []
  const tags = useMemo(() => allTags(all), [all])
  const shown = useMemo(() => sortNotes(filterNotes(all, query, tag)), [all, query, tag])
  const filtering = query.trim() !== '' || tag !== null

  const close = () => setEditing(null)
  const note = editing === 'new' ? null : editing

  /** Sempre `visibility: ''`: nos atalhos do cartão a visibilidade não muda (só o dono muda, e vazio mantém). */
  function quickUpdate(n: Note, patch: Partial<NoteInput>) {
    setNotice(undefined)
    actions.update.mutate(
      { id: n.id, ...withChange(n, patch), visibility: '', version: n.version },
      {
        onError: (err) => {
          setNotice(errorMessage(tc, err))
          void actions.refresh()
        },
      },
    )
  }

  function save(input: NoteInput, version?: number) {
    if (note && version !== undefined) {
      const owner = noteCan(note, me.id).changeVisibility
      actions.update.mutate({ id: note.id, ...input, visibility: owner ? input.visibility : '', version }, { onSuccess: close })
    } else {
      actions.create.mutate(input, { onSuccess: close })
    }
  }

  return (
    <div className={styles.page}>
      <header className={styles.hero}>
        <div>
          <h1 className={styles.title}>{t('title')}</h1>
          {notes.isSuccess && <p className={styles.count}>{t('count', { count: all.length })}</p>}
        </div>
        <Button variant="primary" className={styles.cta} onClick={() => setEditing('new')}>
          {t('new')}
        </Button>
      </header>

      {all.length > 0 && (
        <div className={styles.toolbar}>
          <TextField label={t('search')} type="search" value={query} onChange={(e) => setQuery(e.target.value)} className={styles.search} />
          {tags.length > 0 && (
            <ul className={styles.tags} aria-label={t('filterTags')}>
              {tags.map((name) => (
                <li key={name}>
                  <button type="button" className={styles.tag} aria-pressed={tag === name} onClick={() => setTag(tag === name ? null : name)}>
                    {name}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {notice && (
        <p className={styles.notice} role="alert">
          {notice}
        </p>
      )}

      <div aria-live="polite">
        {notes.isPending && (
          <div className={styles.skeletons}>
            <Skeleton height="9rem" />
            <Skeleton height="9rem" />
          </div>
        )}

        {notes.isError && (
          <EmptyState
            title={t('loadError')}
            body={errorMessage(tc, notes.error)}
            action={<Button onClick={() => void notes.refetch()}>{tc('actions.tryAgain')}</Button>}
          />
        )}

        {notes.isSuccess && all.length === 0 && (
          <EmptyState
            title={t('empty.title')}
            body={t('empty.body')}
            action={
              <Button variant="primary" onClick={() => setEditing('new')}>
                {t('new')}
              </Button>
            }
          />
        )}

        {notes.isSuccess && all.length > 0 && shown.length === 0 && (
          <EmptyState
            title={t('noResults.title')}
            body={t('noResults.body')}
            action={
              filtering ? (
                <Button
                  onClick={() => {
                    setQuery('')
                    setTag(null)
                  }}
                >
                  {t('noResults.clear')}
                </Button>
              ) : undefined
            }
          />
        )}

        {shown.length > 0 && (
          <ul className={styles.grid}>
            {shown.map((n) => {
              const can = noteCan(n, me.id)
              return (
                <NoteCard
                  key={n.id}
                  note={n}
                  mine={n.owner_id === me.id}
                  canEdit={can.edit}
                  onOpen={() => setEditing(n)}
                  onTogglePin={() => quickUpdate(n, { pinned: !n.pinned })}
                  onToggleItem={(i) => quickUpdate(n, { items: (n.items ?? []).map((it, j) => (j === i ? { ...it, done: !it.done } : it)) })}
                />
              )
            })}
          </ul>
        )}
      </div>

      {editing !== null && (
        <NoteEditor
          key={note?.id ?? 'new'}
          open
          onOpenChange={(o) => !o && close()}
          note={note}
          isOwner={!note || noteCan(note, me.id).changeVisibility}
          readOnly={!!note && !noteCan(note, me.id).edit}
          pending={actions.create.isPending || actions.update.isPending}
          error={note ? actions.update.error : actions.create.error}
          onSave={save}
          onDelete={note ? () => actions.remove.mutate(note.id, { onSuccess: close }) : undefined}
          onLoadCurrent={note ? () => notesApi.get(note.id) : undefined}
        />
      )}
    </div>
  )
}
