import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Dialog, DialogClose } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Item, ItemInput } from './api'
import { ItemForm } from './ItemForm'
import styles from './ItemEditor.module.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  item: Item
  pending?: boolean
  error?: unknown
  onSave: (body: ItemInput) => void
  onDelete: () => void
}

/** Edita um item da lista (nome, quantidade, categoria, recorrência) ou o apaga. Monte com `key={item.id}`. */
export function ItemEditor({ open, onOpenChange, item, pending, error, onSave, onDelete }: Props) {
  const { t } = useTranslation('shopping')
  const { t: tc } = useTranslation('common')
  const [confirm, setConfirm] = useState(false)

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t('editor.title')}>
      <ItemForm
        submitLabel={t('editor.save')}
        pending={pending}
        error={
          error != null && (
            <p className={styles.error} role="alert">
              {errorMessage(tc, error)}
            </p>
          )
        }
        initial={{
          name: item.name,
          quantity: item.quantity == null ? '' : String(item.quantity).replace('.', ','),
          unit: item.unit,
          category: item.category,
          recurring: item.recurring,
        }}
        onSubmit={(body) => {
          onSave(body)
        }}
      >
        <Button variant="danger" className={styles.delete} onClick={() => (confirm ? onDelete() : setConfirm(true))}>
          {confirm ? t('editor.confirmDelete') : t('editor.delete')}
        </Button>
        <DialogClose asChild>
          <Button variant="quiet">{tc('actions.cancel')}</Button>
        </DialogClose>
      </ItemForm>
    </Dialog>
  )
}
