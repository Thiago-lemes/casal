/**
 * Cliente HTTP único do app. A sessão é um cookie HttpOnly na mesma origem,
 * então nada de token no JavaScript.
 *
 * Erros viram ApiError com um `code` estável. Enquanto o backend não envia
 * `code`, ele é derivado do status HTTP; a interface traduz por `errors.<code>`
 * e nunca mostra a frase em português que vem do servidor.
 */
export type ErrorCode =
  | 'invalid'
  | 'unauthorized'
  | 'forbidden'
  | 'not_found'
  | 'conflict'
  | 'rate_limited'
  | 'upstream'
  | 'network'
  | 'unknown'

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: ErrorCode,
    readonly detail?: string,
  ) {
    super(code)
  }
}

const byStatus: Record<number, ErrorCode> = {
  400: 'invalid',
  401: 'unauthorized',
  403: 'forbidden',
  404: 'not_found',
  409: 'conflict',
  429: 'rate_limited',
  502: 'upstream',
}

export function codeFromStatus(status: number): ErrorCode {
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
    const body = (data ?? {}) as { code?: ErrorCode; error?: string }
    throw new ApiError(res.status, body.code ?? codeFromStatus(res.status), body.error)
  }
  return data as T
}
