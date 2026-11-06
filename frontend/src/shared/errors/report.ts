/**
 * Relatório de erros do navegador (Sentry/GlitchTip), desligado sem VITE_SENTRY_DSN. O pacote só é baixado quando
 * há DSN. O que sai é só o tipo do erro, a pilha e a rota; nunca o texto da mensagem (pode ter o que a pessoa digitou),
 * a URL com ids, cookies, corpo ou e-mail.
 */

type SentryEvent = {
  request?: unknown
  breadcrumbs?: unknown
  contexts?: unknown
  extra?: unknown
  user?: { id?: string }
  message?: string
  tags?: Record<string, string>
  exception?: { values?: { type?: string; value?: string }[] }
}

/** Troca ids e números no caminho por marcadores, para a rota agrupar e não identificar nada. */
export function routeOf(pathname: string): string {
  return pathname
    .split('/')
    .map((seg) => (/^[0-9a-f]{8}-[0-9a-f-]{27}$/i.test(seg) || /^\d+$/.test(seg) ? ':id' : seg))
    .join('/')
}

/** Última barreira antes do envio. Exportada para o teste provar que nada de conteúdo passa. */
export function scrub<T extends SentryEvent>(event: T, pathname = '/'): T {
  delete event.request
  delete event.breadcrumbs
  delete event.contexts
  delete event.extra
  event.user = event.user?.id ? { id: event.user.id } : undefined
  delete event.message
  for (const v of event.exception?.values ?? []) v.value = undefined // a mensagem pode trazer texto digitado
  event.tags = { ...event.tags, route: routeOf(pathname) }
  return event
}

export async function initErrorReporting(): Promise<void> {
  const dsn = import.meta.env.VITE_SENTRY_DSN as string | undefined
  if (!dsn) return
  const Sentry = await import('@sentry/browser')
  Sentry.init({
    dsn,
    environment: (import.meta.env.VITE_APP_ENV as string | undefined) ?? 'beta',
    release: import.meta.env.VITE_APP_RELEASE as string | undefined,
    defaultIntegrations: false,
    integrations: [Sentry.globalHandlersIntegration(), Sentry.dedupeIntegration()],
    beforeBreadcrumb: () => null,
    beforeSend: (event) => scrub(event as SentryEvent, window.location.pathname) as typeof event,
  })
}
