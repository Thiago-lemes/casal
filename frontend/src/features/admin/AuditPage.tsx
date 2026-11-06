import { useTranslation } from 'react-i18next'
import { EmptyState, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useAudit } from './hooks'
import { when } from './format'
import styles from './Admin.module.css'

/** Últimas ações do painel (somente leitura; a tabela no banco só aceita inserção). */
export function AuditPage() {
  const { t, i18n } = useTranslation('admin')
  const { t: tc } = useTranslation('common')
  const audit = useAudit()

  if (audit.isPending) return <Skeleton />
  if (audit.isError) return <EmptyState title={errorMessage(tc, audit.error)} />
  if (audit.data.length === 0) return <EmptyState title={t('audit.none')} />

  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>{t('audit.when')}</th>
            <th>{t('audit.who')}</th>
            <th>{t('audit.action')}</th>
            <th>{t('audit.change')}</th>
          </tr>
        </thead>
        <tbody>
          {audit.data.map((r) => (
            <tr key={r.id}>
              <td>{when(r.occurred_at, i18n.language)}</td>
              <td>{r.admin_email}</td>
              <td>{t(`audit.actions.${r.action}`, { defaultValue: r.action })}</td>
              <td>{r.before || r.after ? `${JSON.stringify(r.before ?? {})} → ${JSON.stringify(r.after ?? {})}` : ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
