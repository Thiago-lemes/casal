import { api } from '@/shared/api/client'

export type NoteKind = 'text' | 'checklist'
/** `private` só o dono; `view` o parceiro lê; `household` os dois editam. */
export type NoteVisibility = 'private' | 'view' | 'household'

export type NoteItem = { text: string; done: boolean }

export type Note = {
  id: string
  owner_id: string
  owner_name: string
  title: string
  kind: NoteKind
  body: string
  items: NoteItem[] | null
  tags: string[] | null
  visibility: NoteVisibility
  pinned: boolean
  /** a versão que a pessoa viu; o `PUT` exige e responde `note_conflict` se mudou */
  version: number
  created_at: string
  updated_at: string
}

export type NoteInput = {
  title: string
  kind: NoteKind
  body: string
  items: NoteItem[]
  tags: string[]
  visibility: NoteVisibility
  pinned: boolean
}

export const notesApi = {
  list: async () => (await api<Note[] | null>('/v1/notes')) ?? [],
  get: (id: string) => api<Note>(`/v1/notes/${id}`),
  create: (v: NoteInput) => api<Note>('/v1/notes', { method: 'POST', body: v }),
  /** `kind` não muda depois de criada; `visibility` vazio mantém (só o dono muda). */
  update: (id: string, v: Omit<NoteInput, 'visibility'> & { visibility: NoteVisibility | ''; version: number }) =>
    api<Note>(`/v1/notes/${id}`, { method: 'PUT', body: v }),
  remove: (id: string) => api<void>(`/v1/notes/${id}`, { method: 'DELETE' }),
}
