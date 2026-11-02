import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose } from '@/design-system'
import { ApiError } from '@/shared/api/client'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Balance } from '../api'
import { formatDay } from '../dates'
import { useSettle } from '../hooks'
import { formatBRL } from '../money'
import { nameOf } from '../types'
import type { Ctx } from '../types'
import styles from '../finance.module.css'

type Props = {
  ctx: Ctx
  /** o saldo ao vivo da consulta: se o servidor disser que mudou, ele se atualiza e a pessoa confirma de novo */
  balance: Balance
  onReload: () => Promise<unknown> | void
  onClose: () => void
}

/**
 * Registrar o acerto. O app só registra: o pagamento acontece por fora.
 * Envia exatamente o valor e o dia que a pessoa está vendo; se o saldo mudou, recarrega e pede nova confirmação,
 * nunca reenvia sozinho com o valor novo.
 */
export function SettleDialog({ ctx, balance, onReload, onClose }: Props) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const settle = useSettle()
  const [stale, setStale] = useState(false)

  const from = balance.from_user === ctx.me.id ? t('form.me') : nameOf(ctx, balance.from_user ?? '')
  const to = balance.to_user === ctx.me.id ? t('form.me') : nameOf(ctx, balance.to_user ?? '')

  function confirm() {
    setStale(false)
    settle.mutate(
      { through_on: balance.through, amount_cents: balance.amount_cents },
      {
        onSuccess: onClose,
        onError: (err) => {
          if (err instanceof ApiError && (err.is('settlement_stale') || err.is('nothing_to_settle'))) {
            setStale(err.is('settlement_stale'))
            void onReload()
          }
        },
      },
    )
  }

  const error = settle.error instanceof ApiError && settle.error.is('settlement_stale') ? undefined : settle.error

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('settle.title')}>
      <div className={styles.form}>
        {balance.amount_cents > 0 ? (
          <>
            <p>
              <strong>{t('settle.direction', { from, to })}</strong>
            </p>
            <p style={{ fontSize: 'var(--text-xl)', fontWeight: 700 }}>{formatBRL(balance.amount_cents, i18n.language)}</p>
            <p className={styles.hint}>{t('settle.through', { date: formatDay(balance.through, i18n.language, { day: 'numeric', month: 'long' }) })}</p>
          </>
        ) : (
          <p>{t('balance.even')}</p>
        )}

        <p className={styles.notice}>{t('settle.offline')}</p>

        {stale && (
          <p className={styles.notice} role="alert">
            {t('settle.stale')}
          </p>
        )}
        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}

        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button variant="primary" loading={settle.isPending} disabled={balance.amount_cents <= 0} onClick={confirm}>
            {t('settle.confirm')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
