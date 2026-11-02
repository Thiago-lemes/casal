import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/shared/api/client'
import { pushStatus, readEnv, urlBase64ToUint8Array } from './support'
import type { PushStatus } from './support'

const statusKey = ['push-status'] as const

/** Falhas que não vêm da API: o navegador não deixou ou o service worker ainda não existe. */
export class PushError extends Error {
  constructor(readonly reason: 'noWorker' | 'denied' | 'timeout') {
    super(reason)
  }
}

/** Espera o service worker ficar pronto, sem travar para sempre (ele não existe no modo de desenvolvimento). */
async function readyWorker(): Promise<ServiceWorkerRegistration> {
  const timeout = new Promise<never>((_, reject) => setTimeout(() => reject(new PushError('noWorker')), 6000))
  return Promise.race([navigator.serviceWorker.ready, timeout])
}

/** O navegador às vezes nunca responde (sem serviço de push): melhor falhar com uma mensagem que girar para sempre. */
function withTimeout<T>(promise: Promise<T>, ms: number): Promise<T> {
  const timeout = new Promise<never>((_, reject) => setTimeout(() => reject(new PushError('timeout')), ms))
  return Promise.race([promise, timeout])
}

async function subscribe() {
  const reg = await readyWorker()
  if ((await Notification.requestPermission()) !== 'granted') throw new PushError('denied')

  const { public_key } = await api<{ public_key: string }>('/v1/push/vapid-key')
  const sub = await withTimeout(
    reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: urlBase64ToUint8Array(public_key) }),
    20_000,
  )
  await api<void>('/v1/push/subscriptions', { method: 'POST', body: sub.toJSON() })
}

async function unsubscribe() {
  const reg = await navigator.serviceWorker.getRegistration()
  const sub = await reg?.pushManager.getSubscription()
  if (!sub) return
  await api<void>('/v1/push/subscriptions', { method: 'DELETE', body: { endpoint: sub.endpoint } })
  await sub.unsubscribe()
}

export function usePush() {
  const qc = useQueryClient()
  const refresh = () => qc.invalidateQueries({ queryKey: statusKey })

  const status = useQuery<PushStatus>({
    queryKey: statusKey,
    queryFn: async () => pushStatus(await readEnv()),
    staleTime: Infinity,
  })

  return {
    status: status.data,
    enable: useMutation({ mutationFn: subscribe, onSettled: refresh }),
    disable: useMutation({ mutationFn: unsubscribe, onSettled: refresh }),
    test: useMutation({ mutationFn: () => api<void>('/v1/push/test', { method: 'POST' }) }),
  }
}
