/**
 * Cliente HTTP único do app. A sessão é um cookie HttpOnly na mesma origem,
 * então nada de token no JavaScript.
 *
 * Erros viram ApiError com o `code` que o backend envia em toda resposta de erro
 * (429 não traz: o código vem do status). A interface traduz por `errors.<code>`
 * e nunca mostra a frase em português que vem do servidor.
 */

/** Tipos de erro (o do status HTTP); o backend envia também um `code` mais específico (ex.: `task_not_found`). */
export type ErrorKind =
  | 'invalid'
  | 'unauthorized'
  | 'forbidden'
  | 'not_found'
  | 'conflict'
  | 'rate_limited'
  | 'upstream'
  | 'internal'
  | 'network'
  | 'unknown'

export class ApiError extends Error {
  constructor(
    readonly status: number,
    /** O `code` do servidor, ou o tipo derivado do status quando ele não envia (429). */
    readonly code: string,
    readonly detail?: string,
  ) {
    super(code)
  }

  /** O tipo geral do erro, útil como mensagem de reserva quando o `code` específico não tem texto. */
  get kind(): ErrorKind {
    return codeFromStatus(this.status)
  }

  /** Confere o `code` específico (ex.: `err.is('note_conflict')`). */
  is(code: string): boolean {
    return this.code === code
  }
}

const byStatus: Record<number, ErrorKind> = {
  400: 'invalid',
  401: 'unauthorized',
  403: 'forbidden',
  404: 'not_found',
  409: 'conflict',
  429: 'rate_limited',
  500: 'internal',
  502: 'upstream',
}

export function codeFromStatus(status: number): ErrorKind {
  if (status === 0) return 'network'
  return byStatus[status] ?? 'unknown'
}

export async function api<T>(path: string, init: { method?: string; body?: unknown } = {}): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method: init.method ?? 'GET',
      headers: init.body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
      credentials: 'same-origin',
    })
  } catch {
    throw new ApiError(0, 'network')
  }

  if (res.status === 204) return undefined as T
  const data: unknown = await res.json().catch(() => null)
  if (!res.ok) {
    const body = (data ?? {}) as { code?: string; error?: string }
    throw new ApiError(res.status, body.code ?? codeFromStatus(res.status), body.error)
  }
  return data as T
}
