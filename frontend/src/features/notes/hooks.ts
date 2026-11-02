import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { notesApi } from './api'
import type { NoteInput } from './api'

export const notesKey = ['notes'] as const

/** Todas as anotações que a pessoa vê; a busca e as etiquetas filtram na tela (ver `filterNotes`). */
export function useNotes() {
  return useQuery({ queryKey: notesKey, queryFn: notesApi.list })
}

export function useNoteActions() {
  const qc = useQueryClient()
  const refresh = () => qc.invalidateQueries({ queryKey: notesKey })

  return {
    refresh,
    create: useMutation({ mutationFn: (v: NoteInput) => notesApi.create(v), onSuccess: refresh }),
    update: useMutation({
      mutationFn: (v: { id: string } & Parameters<typeof notesApi.update>[1]) => {
        const { id, ...body } = v
        return notesApi.update(id, body)
      },
      onSuccess: refresh,
    }),
    remove: useMutation({ mutationFn: notesApi.remove, onSuccess: refresh }),
  }
}

export type NoteActionsApi = ReturnType<typeof useNoteActions>
