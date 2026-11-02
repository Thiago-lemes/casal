import type { Note, NoteInput } from './api'

/** O que a pessoa pode fazer com uma anotação (o backend continua sendo a autoridade). */
export function noteCan(note: Note, meId: string) {
  const owner = note.owner_id === meId
  return {
    edit: owner || note.visibility === 'household',
    /** só o dono muda a visibilidade e apaga */
    changeVisibility: owner,
    remove: owner,
  }
}

const norm = (s: string) => s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()

/** Busca no título, no texto e nos itens, sem diferenciar maiúsculas nem acentos; e por etiqueta. */
export function filterNotes(notes: Note[], query: string, tag: string | null): Note[] {
  const q = norm(query.trim())
  return notes.filter((n) => {
    if (tag && !(n.tags ?? []).includes(tag)) return false
    if (!q) return true
    const haystack = [n.title, n.body, ...(n.items ?? []).map((i) => i.text), ...(n.tags ?? [])].map(norm)
    return haystack.some((h) => h.includes(q))
  })
}

/** Fixadas primeiro; dentro de cada grupo, as alteradas há menos tempo antes. */
export function sortNotes(notes: Note[]): Note[] {
  return [...notes].sort((a, b) => Number(b.pinned) - Number(a.pinned) || b.updated_at.localeCompare(a.updated_at))
}

/** Etiquetas de todas as anotações, em ordem alfabética, sem repetir. */
export function allTags(notes: Note[]): string[] {
  return [...new Set(notes.flatMap((n) => n.tags ?? []))].sort((a, b) => a.localeCompare(b))
}

/** "mercado, casa ,, Casa" vira ["mercado", "casa"]: sem vazias e sem repetir (sem diferenciar maiúsculas). */
export function parseTags(text: string): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of text.split(',')) {
    const tag = raw.trim()
    if (tag && !seen.has(tag.toLowerCase())) {
      seen.add(tag.toLowerCase())
      out.push(tag)
    }
  }
  return out
}

/** Itens do rascunho sem as linhas em branco. */
export const cleanItems = (items: { text: string; done: boolean }[]) =>
  items.map((i) => ({ text: i.text.trim(), done: i.done })).filter((i) => i.text)

/** O corpo que o `PUT` espera a partir de uma anotação existente com uma mudança. */
export function withChange(note: Note, patch: Partial<NoteInput>): NoteInput {
  return {
    title: note.title,
    kind: note.kind,
    body: note.body,
    items: note.items ?? [],
    tags: note.tags ?? [],
    visibility: note.visibility,
    pinned: note.pinned,
    ...patch,
  }
}
