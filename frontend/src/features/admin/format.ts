/** Data e hora curtas no idioma da interface; "—" quando não há data. */
export function when(iso: string | null | undefined, lang: string): string {
  if (!iso) return '—'
  return new Intl.DateTimeFormat(lang, { dateStyle: 'short', timeStyle: 'short' }).format(new Date(iso))
}

/** Estado de um código de cadastro, para a lista. */
export function codeState(c: { revoked_at?: string | null; expires_at: string; uses: number; max_uses: number }, now = Date.now()) {
  if (c.revoked_at) return 'revoked'
  if (new Date(c.expires_at).getTime() <= now) return 'expired'
  if (c.uses >= c.max_uses) return 'used'
  return 'active'
}
