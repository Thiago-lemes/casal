/// <reference lib="webworker" />
import { precacheAndRoute, cleanupOutdatedCaches } from 'workbox-precaching'

declare const self: ServiceWorkerGlobalScope

cleanupOutdatedCaches()
precacheAndRoute(self.__WB_MANIFEST)

self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') void self.skipWaiting()
})

// Web Push: no iOS toda mensagem precisa mostrar uma notificação.
self.addEventListener('push', (event) => {
  let data = { title: 'Zoog', body: '', url: '/' }
  try {
    data = { ...data, ...event.data?.json() }
  } catch {
    /* corpo vazio ou inválido: usa o padrão */
  }
  event.waitUntil(
    self.registration.showNotification(data.title, { body: data.body, data: { url: data.url } }),
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = (event.notification.data as { url?: string })?.url || '/'
  event.waitUntil(self.clients.openWindow(url))
})
