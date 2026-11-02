import { Check, Maximize2, Minimize2, Repeat, RotateCcw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Card, EmptyState, Skeleton } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Item, ItemInput } from './api'
import { useShopping, useShoppingActions } from './hooks'
import { ItemEditor } from './ItemEditor'
import { ItemForm } from './ItemForm'
import { boughtRecently, formatQuantity, groupOpen } from './rules'
import styles from './ShoppingPage.module.css'

type Props = {
  /** com o módulo Finanças ligado, o modo mercado oferece registrar o gasto da compra */
  onRegisterExpense?: () => void
}

export function ShoppingPage({ onRegisterExpense }: Props = {}) {
  const { t, i18n } = useTranslation('shopping')
  const { t: tc } = useTranslation('common')
  const list = useShopping()
  const actions = useShoppingActions()
  const [editing, setEditing] = useState<Item | null>(null)
  const [market, setMarket] = useState(false)
  const [notice, setNotice] = useState<string>()

  const items = list.data ?? []
  const groups = groupOpen(items)
  const bought = boughtRecently(items)
  const openCount = groups.reduce((n, g) => n + g.items.length, 0)

  function add(body: ItemInput) {
    setNotice(undefined)
    actions.create.mutate(body, {
      onSuccess: (res) => setNotice(res.merged ? t('notice.merged', { name: res.name }) : undefined),
      onError: (err) => setNotice(errorMessage(tc, err)),
    })
    return true
  }

  function restock() {
    setNotice(undefined)
    actions.restock.mutate(undefined, {
      onSuccess: (res) => setNotice(t('notice.restocked', { count: res.restocked })),
      onError: (err) => setNotice(errorMessage(tc, err)),
    })
  }

  function mark(item: Item, done: boolean) {
    setNotice(undefined)
    const m = done ? actions.bought : actions.unbought
    m.mutate(item.id, { onError: (err) => setNotice(errorMessage(tc, err)) })
  }

  const row = (item: Item, isBought: boolean) => (
    <li key={item.id} className={styles.row} data-bought={isBought || undefined}>
      <button
        type="button"
        className={styles.check}
        data-checked={isBought || undefined}
        aria-label={isBought ? t('row.undo', { name: item.name }) : t('row.buy', { name: item.name })}
        onClick={() => mark(item, !isBought)}
      >
        {isBought ? <Check size={22} aria-hidden="true" /> : null}
      </button>

      {market ? (
        <span className={styles.name}>{item.name}</span>
      ) : (
        <button type="button" className={styles.name} onClick={() => setEditing(item)} aria-label={t('row.edit', { name: item.name })}>
          {item.name}
        </button>
      )}

      <span className={styles.qty}>{formatQuantity(item.quantity, item.unit, i18n.language)}</span>
      {item.recurring && !market && <Repeat size={16} className={styles.recurring} role="img" aria-label={t('row.recurring')} />}
    </li>
  )

  return (
    <div className={styles.page} data-market={market || undefined}>
      <header className={styles.hero}>
        <div>
          <h1 className={styles.title}>{market ? t('market.title') : t('title')}</h1>
          {list.isSuccess && <p className={styles.count}>{t('count', { count: openCount })}</p>}
        </div>
        <Button variant="primary" className={styles.cta} onClick={() => setMarket(!market)} aria-pressed={market}>
          {market ? <Minimize2 size={18} aria-hidden="true" /> : <Maximize2 size={18} aria-hidden="true" />}
          {market ? t('market.exit') : t('market.enter')}
        </Button>
      </header>

      {!market && (
        <Card title={t('add.title')}>
          <ItemForm expanded={false} submitLabel={t('add.submit')} pending={actions.create.isPending} onSubmit={add} />
        </Card>
      )}

      {notice && (
        <p className={styles.notice} role="status">
          {notice}
        </p>
      )}

      <div aria-live="polite" className={styles.lists}>
        {list.isPending && (
          <div className={styles.skeletons}>
            <Skeleton height="8rem" />
          </div>
        )}

        {list.isError && (
          <EmptyState
            title={t('loadError')}
            body={errorMessage(tc, list.error)}
            action={<Button onClick={() => void list.refetch()}>{tc('actions.tryAgain')}</Button>}
          />
        )}

        {list.isSuccess && groups.length === 0 && (
          <EmptyState
            title={bought.length > 0 ? t('empty.done.title') : t('empty.title')}
            body={bought.length > 0 ? t('empty.done.body') : t('empty.body')}
          />
        )}

        {groups.map((g) => (
          <Card key={g.category} title={t(`category.${g.category}`)} className={styles.group}>
            <ul>{g.items.map((i) => row(i, false))}</ul>
          </Card>
        ))}

        {bought.length > 0 && (
          <Card title={t('bought.title')} className={styles.group}>
            <ul>{bought.map((i) => row(i, true))}</ul>
          </Card>
        )}
      </div>

      {market && onRegisterExpense && (
        <div className={styles.restock}>
          <Button variant="primary" onClick={onRegisterExpense}>
            {t('market.registerExpense')}
          </Button>
        </div>
      )}

      {!market && list.isSuccess && (
        <div className={styles.restock}>
          <Button onClick={restock} loading={actions.restock.isPending}>
            <RotateCcw size={18} aria-hidden="true" />
            {t('restock.action')}
          </Button>
          <p className={styles.hint}>{t('restock.hint')}</p>
        </div>
      )}

      {editing && (
        <ItemEditor
          key={editing.id}
          open
          onOpenChange={(o) => !o && setEditing(null)}
          item={editing}
          pending={actions.update.isPending}
          error={actions.update.error}
          onSave={(body) => actions.update.mutate({ id: editing.id, body }, { onSuccess: () => setEditing(null) })}
          onDelete={() => actions.remove.mutate(editing.id, { onSuccess: () => setEditing(null) })}
        />
      )}
    </div>
  )
}
