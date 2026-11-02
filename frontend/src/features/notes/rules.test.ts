import { describe, expect, it } from 'vitest'
import type { Note } from './api'
import { allTags, cleanItems, filterNotes, noteCan, parseTags, sortNotes, withChange } from './rules'

const note = (over: Partial<Note>): Note => ({
  id: 'n',
  owner_id: 'me',
  owner_name: 'Ana',
  title: 'Título',
  kind: 'text',
  body: '',
  items: null,
  tags: null,
  visibility: 'private',
  pinned: false,
  version: 1,
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-01T10:00:00Z',
  ...over,
})

describe('noteCan', () => {
  it('o dono edita, muda a visibilidade e apaga', () => {
    expect(noteCan(note({}), 'me')).toEqual({ edit: true, changeVisibility: true, remove: true })
  })

  it('o parceiro só lê uma anotação "view"', () => {
    expect(noteCan(note({ owner_id: 'other', visibility: 'view' }), 'me')).toEqual({ edit: false, changeVisibility: false, remove: false })
  })

  it('em uma anotação do casal o parceiro edita, mas não muda a visibilidade nem apaga', () => {
    expect(noteCan(note({ owner_id: 'other', visibility: 'household' }), 'me')).toEqual({
      edit: true,
      changeVisibility: false,
      remove: false,
    })
  })
})

describe('filterNotes', () => {
  const notes = [
    note({ id: 'a', title: 'Receita de feijão', tags: ['cozinha'] }),
    note({ id: 'b', title: 'Mercado', kind: 'checklist', items: [{ text: 'Açúcar', done: false }], tags: ['casa', 'mercado'] }),
    note({ id: 'c', title: 'Ideias', body: 'viagem para o sul' }),
  ]

  it('busca no título, no texto e nos itens, sem diferenciar acento', () => {
    expect(filterNotes(notes, 'feijao', null).map((n) => n.id)).toEqual(['a'])
    expect(filterNotes(notes, 'acucar', null).map((n) => n.id)).toEqual(['b'])
    expect(filterNotes(notes, 'VIAGEM', null).map((n) => n.id)).toEqual(['c'])
  })

  it('filtra por etiqueta e combina com a busca', () => {
    expect(filterNotes(notes, '', 'casa').map((n) => n.id)).toEqual(['b'])
    expect(filterNotes(notes, 'feijao', 'casa')).toEqual([])
  })

  it('sem busca nem etiqueta, devolve tudo', () => {
    expect(filterNotes(notes, '  ', null)).toHaveLength(3)
  })
})

describe('sortNotes', () => {
  it('fixadas primeiro; depois as mais recentes', () => {
    const sorted = sortNotes([
      note({ id: 'velha', updated_at: '2026-10-01T00:00:00Z' }),
      note({ id: 'nova', updated_at: '2026-11-01T00:00:00Z' }),
      note({ id: 'fixada', pinned: true, updated_at: '2026-09-01T00:00:00Z' }),
    ])
    expect(sorted.map((n) => n.id)).toEqual(['fixada', 'nova', 'velha'])
  })
})

describe('etiquetas e itens', () => {
  it('parseTags tira vazias e repetidas (sem diferenciar maiúsculas)', () => {
    expect(parseTags('mercado, casa ,, Casa')).toEqual(['mercado', 'casa'])
    expect(parseTags('')).toEqual([])
  })

  it('allTags junta e ordena', () => {
    expect(allTags([note({ tags: ['b', 'a'] }), note({ tags: ['a', 'c'] }), note({})])).toEqual(['a', 'b', 'c'])
  })

  it('cleanItems apara e remove linhas em branco, mantendo o estado de feito', () => {
    expect(cleanItems([{ text: ' leite ', done: true }, { text: '   ', done: false }])).toEqual([{ text: 'leite', done: true }])
  })

  it('withChange troca só o que mudou', () => {
    const n = note({ title: 'X', pinned: false, tags: ['a'] })
    expect(withChange(n, { pinned: true })).toMatchObject({ title: 'X', pinned: true, tags: ['a'], items: [] })
  })
})
