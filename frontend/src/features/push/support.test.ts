import { describe, expect, it } from 'vitest'
import { pushStatus, urlBase64ToUint8Array } from './support'
import type { PushEnv } from './support'

const desktop: PushEnv = {
  userAgent: 'Mozilla/5.0 (Windows NT 10.0) Chrome/130',
  platform: 'Win32',
  maxTouchPoints: 0,
  standalone: false,
  hasServiceWorker: true,
  hasPushManager: true,
  hasNotification: true,
  permission: 'default',
  subscribed: false,
}
const iphone: PushEnv = { ...desktop, userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)', platform: 'iPhone' }

describe('pushStatus', () => {
  it('permissão ainda não pedida: desligado', () => {
    expect(pushStatus(desktop)).toBe('off')
  })

  it('permissão dada e assinatura feita: ligado', () => {
    expect(pushStatus({ ...desktop, permission: 'granted', subscribed: true })).toBe('on')
  })

  it('permissão dada mas sem assinatura (limpou os dados): desligado', () => {
    expect(pushStatus({ ...desktop, permission: 'granted', subscribed: false })).toBe('off')
  })

  it('permissão negada: bloqueado', () => {
    expect(pushStatus({ ...desktop, permission: 'denied' })).toBe('denied')
  })

  it('iPhone no Safari (fora da tela inicial): pede instalação, mesmo sem PushManager', () => {
    expect(pushStatus({ ...iphone, hasPushManager: false })).toBe('install')
  })

  it('iPhone com o app instalado: segue a regra normal', () => {
    expect(pushStatus({ ...iphone, standalone: true })).toBe('off')
  })

  it('iPad que se identifica como Mac também conta como iOS', () => {
    const ipad = { ...desktop, platform: 'MacIntel', maxTouchPoints: 5 }
    expect(pushStatus(ipad)).toBe('install')
  })

  it('sem PushManager em outro navegador: sem suporte', () => {
    expect(pushStatus({ ...desktop, hasPushManager: false })).toBe('unsupported')
  })
})

describe('urlBase64ToUint8Array', () => {
  it('converte base64url com e sem preenchimento', () => {
    expect([...urlBase64ToUint8Array('AQID')]).toEqual([1, 2, 3])
    expect([...urlBase64ToUint8Array('-_8')]).toEqual([251, 255])
  })
})
