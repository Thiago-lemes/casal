/** Em que situação o aparelho está para receber avisos. */
export type PushStatus = 'unsupported' | 'install' | 'denied' | 'off' | 'on'

export type PushEnv = {
  userAgent: string
  platform: string
  maxTouchPoints: number
  standalone: boolean
  hasServiceWorker: boolean
  hasPushManager: boolean
  hasNotification: boolean
  permission: NotificationPermission
  subscribed: boolean
}

export const isIOS = (e: Pick<PushEnv, 'userAgent' | 'platform' | 'maxTouchPoints'>) =>
  /iphone|ipad|ipod/i.test(e.userAgent) || (e.platform === 'MacIntel' && e.maxTouchPoints > 1)

/** Decide o estado a partir do ambiente (função pura, para poder testar). */
export function pushStatus(e: PushEnv): PushStatus {
  // no iPhone o push só existe com o app instalado na tela inicial
  if (isIOS(e) && !e.standalone) return 'install'
  if (!e.hasServiceWorker || !e.hasPushManager || !e.hasNotification) return 'unsupported'
  if (e.permission === 'denied') return 'denied'
  return e.permission === 'granted' && e.subscribed ? 'on' : 'off'
}

export async function readEnv(): Promise<PushEnv> {
  const hasServiceWorker = 'serviceWorker' in navigator
  const hasPushManager = 'PushManager' in window
  const hasNotification = 'Notification' in window

  let subscribed = false
  if (hasServiceWorker && hasPushManager) {
    const reg = await navigator.serviceWorker.getRegistration()
    subscribed = Boolean(await reg?.pushManager.getSubscription())
  }

  return {
    userAgent: navigator.userAgent,
    platform: navigator.platform,
    maxTouchPoints: navigator.maxTouchPoints,
    standalone:
      matchMedia('(display-mode: standalone)').matches || (navigator as Navigator & { standalone?: boolean }).standalone === true,
    hasServiceWorker,
    hasPushManager,
    hasNotification,
    permission: hasNotification ? Notification.permission : 'default',
    subscribed,
  }
}

/** A chave pública VAPID vem em base64url; o navegador quer bytes. */
export function urlBase64ToUint8Array(base64: string): Uint8Array<ArrayBuffer> {
  const pad = '='.repeat((4 - (base64.length % 4)) % 4)
  const raw = atob((base64 + pad).replace(/-/g, '+').replace(/_/g, '/'))
  return Uint8Array.from(raw, (c) => c.charCodeAt(0))
}
