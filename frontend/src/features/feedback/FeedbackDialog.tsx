import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation } from 'react-router-dom'
import { Button, Dialog, DialogClose, TextArea } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { feedbackApi } from './api'
import { screenFromPath } from './screens'
import styles from './FeedbackDialog.module.css'

export const MAX_FEEDBACK = 2000

/**
 * Mensagem para o time do app. A tela avisa que quem lê somos nós: é a única parte do app cujo texto o time vê,
 * porque a pessoa o escreveu para isso.
 */
export function FeedbackDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation('feedback')
  const { t: tc } = useTranslation('common')
  const { pathname } = useLocation()
  const [message, setMessage] = useState('')
  const send = useMutation({ mutationFn: (text: string) => feedbackApi.send(text, screenFromPath(pathname)) })
  const trimmed = message.trim()

  function close(next: boolean) {
    onOpenChange(next)
    if (!next) {
      // limpa ao fechar, para a próxima abertura começar em branco
      setMessage('')
      send.reset()
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (trimmed) send.mutate(trimmed)
  }

  return (
    <Dialog open={open} onOpenChange={close} title={t('title')}>
      {send.isSuccess ? (
        <div className={styles.form}>
          <p className={styles.lead} role="status">
            {t('thanks')}
          </p>
          <div className={styles.footer}>
            <DialogClose asChild>
              <Button variant="primary">{tc('actions.close')}</Button>
            </DialogClose>
          </div>
        </div>
      ) : (
        <form className={styles.form} onSubmit={submit}>
          <p className={styles.lead}>{t('lead')}</p>
          <TextArea
            label={t('message')}
            value={message}
            maxLength={MAX_FEEDBACK}
            rows={5}
            onChange={(e) => setMessage(e.target.value)}
            autoFocus
          />
          <p className={styles.notice}>{t('notice')}</p>

          {send.isError && (
            <p className={styles.error} role="alert">
              {errorMessage(tc, send.error)}
            </p>
          )}

          <div className={styles.footer}>
            <DialogClose asChild>
              <Button variant="quiet">{tc('actions.cancel')}</Button>
            </DialogClose>
            <Button type="submit" variant="primary" loading={send.isPending} disabled={!trimmed}>
              {t('submit')}
            </Button>
          </div>
        </form>
      )}
    </Dialog>
  )
}
