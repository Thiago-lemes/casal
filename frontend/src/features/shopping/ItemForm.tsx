import { useState } from 'react'
import type { FormEvent, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, TextField } from '@/design-system'
import { SHOPPING_CATEGORIES } from './api'
import type { ItemInput, ShoppingCategory } from './api'
import { parseQuantity, UNIT_SUGGESTIONS } from './rules'
import styles from './ItemForm.module.css'

type Initial = { name: string; quantity: string; unit: string; category: ShoppingCategory; recurring: boolean }

const BLANK: Initial = { name: '', quantity: '', unit: '', category: 'grocery', recurring: false }

type Props = {
  initial?: Initial
  submitLabel: string
  pending?: boolean
  /** chamado com o item pronto; devolva `true` para limpar o formulário (usado ao adicionar vários seguidos) */
  onSubmit: (item: ItemInput) => boolean | void
  /** mostra quantidade, unidade, categoria e "sempre comprar" (no cadastro rápido ficam escondidos até pedir) */
  expanded?: boolean
  /** mensagem de erro, mostrada acima dos botões */
  error?: ReactNode
  /** botões extras (apagar, cancelar), à esquerda do botão de salvar */
  children?: ReactNode
}

/** Os campos de um item da lista; serve para adicionar e para editar. */
export function ItemForm({ initial = BLANK, submitLabel, pending, onSubmit, expanded = true, error, children }: Props) {
  const { t } = useTranslation('shopping')
  const [v, setV] = useState<Initial>(initial)
  const [more, setMore] = useState(expanded)
  const quantity = parseQuantity(v.quantity)

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!v.name.trim() || quantity === undefined) return
    const clear = onSubmit({ name: v.name.trim(), quantity, unit: v.unit.trim(), category: v.category, recurring: v.recurring })
    if (clear) setV(BLANK)
  }

  return (
    <form className={styles.form} onSubmit={submit} noValidate>
      <div className={styles.top}>
        <TextField label={t('form.name')} value={v.name} onChange={(e) => setV({ ...v, name: e.target.value })} maxLength={120} required className={styles.name} />
        {!expanded && (
          <Button type="submit" variant="primary" loading={pending} disabled={!v.name.trim() || quantity === undefined}>
            {submitLabel}
          </Button>
        )}
      </div>

      {!expanded && (
        <div>
          <Button variant="quiet" onClick={() => setMore(!more)} aria-expanded={more}>
            {more ? t('form.less') : t('form.more')}
          </Button>
        </div>
      )}

      {more && (
        <>
          <div className={styles.row}>
            <TextField
              label={t('form.quantity')}
              inputMode="decimal"
              value={v.quantity}
              onChange={(e) => setV({ ...v, quantity: e.target.value })}
              error={quantity === undefined ? t('form.quantityInvalid') : undefined}
            />
            <div className={styles.unit}>
              <TextField label={t('form.unit')} list="shopping-units" value={v.unit} onChange={(e) => setV({ ...v, unit: e.target.value })} maxLength={12} />
              <datalist id="shopping-units">
                {UNIT_SUGGESTIONS.map((u) => (
                  <option key={u} value={u} />
                ))}
              </datalist>
            </div>
          </div>

          <div className={styles.field}>
            <label htmlFor="shopping-category" className={styles.label}>
              {t('form.category')}
            </label>
            <select id="shopping-category" className={styles.select} value={v.category} onChange={(e) => setV({ ...v, category: e.target.value as ShoppingCategory })}>
              {SHOPPING_CATEGORIES.map((c) => (
                <option key={c} value={c}>
                  {t(`category.${c}`)}
                </option>
              ))}
            </select>
          </div>

          <label className={styles.check}>
            <input type="checkbox" checked={v.recurring} onChange={(e) => setV({ ...v, recurring: e.target.checked })} />
            <span>
              {t('form.recurring')}
              <small>{t('form.recurringHint')}</small>
            </span>
          </label>
        </>
      )}

      {error}

      {expanded && (
        <div className={styles.submit}>
          {children}
          <Button type="submit" variant="primary" loading={pending} disabled={!v.name.trim() || quantity === undefined}>
            {submitLabel}
          </Button>
        </div>
      )}
    </form>
  )
}
