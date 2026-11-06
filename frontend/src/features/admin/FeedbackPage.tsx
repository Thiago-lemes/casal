import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Button, EmptyState, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useFeedbackList, useReadFeedback } from './hooks'
import { when } from './format'
import styles from './Admin.module.css'

/** Mensagens que as pessoas escreveram para o time: o único conteúdo que o painel mostra. */
export function FeedbackPage() {
  const { t, i18n } = useTranslation('admin')
  const { t: tc } = useTranslation('common')
  const [unread, setUnread] = useState(true)
  const list = useFeedbackList(unread)
  const read = useReadFeedback()

  return (
    <>
      <label className={styles.toggle}>
        <input type="checkbox" checked={unread} onChange={(e) => setUnread(e.target.checked)} />
        {t('feedback.onlyUnread')}
      </label>
      {list.isPending && <Skeleton />}
      {list.isError && <EmptyState title={errorMessage(tc, list.error)} />}
      {list.data && list.data.length === 0 && <EmptyState title={t('feedback.none')} />}
      <ul className={styles.list}>
        {list.data?.map((f) => (
          <li key={f.id} className={styles.item}>
            <div className={styles.itemHead}>
              <strong>{f.author}</strong>
              <span className={styles.muted}>
                {f.screen ? `${t(`feedback.screens.${f.screen}`, { defaultValue: f.screen })} · ` : ''}
                {when(f.created_at, i18n.language)}
              </span>
            </div>
            <p className={styles.message}>{f.message}</p>
            <div className={styles.row}>
              <Link to={`/admin/casais/${f.household_id}`}>{t('feedback.household')}</Link>
              {!f.read_at && (
                <Button variant="quiet" loading={read.isPending && read.variables === f.id} onClick={() => read.mutate(f.id)}>
                  {t('feedback.markRead')}
                </Button>
              )}
            </div>
          </li>
        ))}
      </ul>
    </>
  )
}
