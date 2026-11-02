import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Card, Skeleton, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Category } from '../api'
import { categoryName } from '../categories'
import { useCategories, useCategoryActions } from '../hooks'
import styles from '../finance.module.css'

/** Criar, renomear, arquivar e desarquivar. Categoria nunca é apagada: os gastos antigos continuam apontando para ela. */
export function CategoriesManager() {
  const { t } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const list = useCategories(true)
  const actions = useCategoryActions()
  const [name, setName] = useState('')
  const [renaming, setRenaming] = useState<{ id: string; name: string }>()

  const error = actions.create.error ?? actions.rename.error ?? actions.archive.error ?? actions.unarchive.error

  return (
    <Card title={t('categories.title')}>
      <form
        className={styles.form}
        onSubmit={(e) => {
          e.preventDefault()
          if (name.trim()) actions.create.mutate(name.trim(), { onSuccess: () => setName('') })
        }}
      >
        <TextField label={t('categories.new')} value={name} maxLength={40} onChange={(e) => setName(e.target.value)} />
        <div>
          <Button type="submit" variant="primary" loading={actions.create.isPending} disabled={!name.trim()}>
            {t('categories.add')}
          </Button>
        </div>
      </form>

      {error != null && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, error)}
        </p>
      )}

      {list.isPending && <Skeleton height="6rem" />}
      <ul className={styles.list}>
        {(list.data ?? []).map((c: Category) => (
          <li key={c.id} className={styles.item}>
            {renaming?.id === c.id ? (
              <form
                className={styles.form}
                onSubmit={(e) => {
                  e.preventDefault()
                  if (renaming.name.trim()) actions.rename.mutate({ id: c.id, name: renaming.name.trim() }, { onSuccess: () => setRenaming(undefined) })
                }}
              >
                <TextField label={t('categories.rename')} value={renaming.name} maxLength={40} onChange={(e) => setRenaming({ id: c.id, name: e.target.value })} />
                <div className={styles.footer}>
                  <Button variant="quiet" onClick={() => setRenaming(undefined)}>
                    {tc('actions.cancel')}
                  </Button>
                  <Button type="submit" variant="primary" loading={actions.rename.isPending}>
                    {t('categories.save')}
                  </Button>
                </div>
              </form>
            ) : (
              <>
                <span className={styles.itemMain}>
                  <span className={styles.itemTitle}>{categoryName(t, c)}</span>
                  {c.archived && <span className={styles.itemMeta}>{t('categories.archivedTag')}</span>}
                </span>
                <span className={styles.segmented}>
                  {!c.key && (
                    <Button variant="quiet" onClick={() => setRenaming({ id: c.id, name: c.name })} aria-label={t('categories.renameOf', { name: categoryName(t, c) })}>
                      {t('categories.rename')}
                    </Button>
                  )}
                  {c.archived ? (
                    <Button variant="quiet" onClick={() => actions.unarchive.mutate(c.id)} aria-label={t('categories.unarchiveOf', { name: categoryName(t, c) })}>
                      {t('categories.unarchive')}
                    </Button>
                  ) : (
                    <Button variant="quiet" onClick={() => actions.archive.mutate(c.id)} aria-label={t('categories.archiveOf', { name: categoryName(t, c) })}>
                      {t('categories.archive')}
                    </Button>
                  )}
                </span>
              </>
            )}
          </li>
        ))}
      </ul>
    </Card>
  )
}
