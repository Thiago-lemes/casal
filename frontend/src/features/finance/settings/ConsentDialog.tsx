import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import styles from '../finance.module.css'

/**
 * Aparece antes de qualquer chamada de consentimento. Quem sabe a própria renda e vê o percentual de cada gasto
 * consegue calcular a renda do outro, então a concordância tem de ser explícita e de ambos.
 */
export function ConsentDialog({
  onConfirm,
  onClose,
  pending,
  error,
}: {
  onConfirm: () => void
  onClose: () => void
  pending?: boolean
  error?: unknown
}) {
  const { t } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('consent.title')}>
      <div className={styles.form}>
        <p>{t('consent.body')}</p>
        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}
        <div className={styles.footer}>
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.cancel')}</Button>
          </DialogClose>
          <Button variant="primary" loading={pending} onClick={onConfirm}>
            {t('consent.agree')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
