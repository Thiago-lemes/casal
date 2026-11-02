import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, ChoiceGroup, Dialog, DialogClose, Skeleton, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Group, Split } from '../api'
import { categoryName, selectable } from '../categories'
import { formatDay } from '../dates'
import { useCategories, useExpenseActions, useGroup } from '../hooks'
import { formatBRL } from '../money'
import { equalSplit } from '../split'
import type { Ctx } from '../types'
import { SplitEditor } from './SplitEditor'
import styles from '../finance.module.css'
import local from './Expenses.module.css'

/** A compra parcelada inteira: as parcelas em ordem, quais já foram acertadas, e editar ou apagar as abertas. */
export function GroupDialog({ groupId, ctx, onClose }: { groupId: string; ctx: Ctx; onClose: () => void }) {
  const { t, i18n } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const group = useGroup(groupId)

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('group.title')}>
      {group.isPending && <Skeleton height="8rem" />}
      {group.isError && (
        <div className={styles.form}>
          <p className={styles.error} role="alert">
            {errorMessage(tc, group.error)}
          </p>
          <Button onClick={() => void group.refetch()}>{tc('actions.tryAgain')}</Button>
        </div>
      )}
      {group.data && <GroupBody group={group.data} ctx={ctx} lang={i18n.language} onClose={onClose} />}
    </Dialog>
  )
}

function GroupBody({ group, ctx, lang, onClose }: { group: Group; ctx: Ctx; lang: string; onClose: () => void }) {
  const { t } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const actions = useExpenseActions()
  const categories = useCategories(true)
  const first = group.expenses[0]
  const openCount = group.expenses.length - group.settled
  const total = group.expenses.reduce((n, e) => n + e.amount_cents, 0)

  const [editing, setEditing] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [description, setDescription] = useState(first?.description ?? '')
  const [categoryId, setCategoryId] = useState(first?.category_id ?? '')
  const [paidBy, setPaidBy] = useState(first?.paid_by ?? ctx.me.id)
  const [splitMode, setSplitMode] = useState<'keep' | 'equal' | 'custom'>('keep')
  const [custom, setCustom] = useState<Split>(first?.split ?? equalSplit(ctx.people))
  const [done, setDone] = useState<string>()

  const list = selectable(categories.data ?? [], first?.category_id)

  function save() {
    const body = {
      description: description.trim(),
      category_id: categoryId || first?.category_id,
      paid_by: paidBy,
      ...(splitMode === 'equal' ? { split: equalSplit(ctx.people) } : splitMode === 'custom' ? { split: custom } : {}),
    }
    actions.updateGroup.mutate({ id: group.group_id, body }, { onSuccess: () => setEditing(false) })
  }

  function remove() {
    actions.removeGroup.mutate(group.group_id, {
      onSuccess: (res) => {
        if (res.remaining === 0) onClose()
        else {
          setConfirming(false)
          setDone(t('group.deleted', { deleted: res.deleted, remaining: res.remaining }))
        }
      },
    })
  }

  const error = actions.updateGroup.error ?? actions.removeGroup.error

  return (
    <div className={styles.form}>
      <p className={local.total}>{formatBRL(total, lang)}</p>
      <ul className={styles.list} aria-label={t('group.installments')}>
        {group.expenses.map((e) => (
          <li key={e.id} className={styles.item}>
            <span className={styles.itemMain}>
              <span className={styles.itemTitle}>{t('row.installment', { no: e.installment?.no ?? 0, total: e.installment?.total ?? group.expenses.length })}</span>
              <span className={styles.itemMeta}>
                <span>{formatDay(e.spent_on, lang, { day: 'numeric', month: 'short', year: 'numeric' })}</span>
                {e.settled && <span className={styles.badge}>{t('row.settled')}</span>}
              </span>
            </span>
            <span className={styles.money}>{formatBRL(e.amount_cents, lang)}</span>
          </li>
        ))}
      </ul>

      {done && (
        <p className={styles.notice} role="status">
          {done}
        </p>
      )}
      {error != null && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, error)}
        </p>
      )}

      {editing ? (
        <div className={styles.form}>
          <p className={styles.hint}>{t('group.editHint')}</p>
          <TextField label={t('form.description')} value={description} maxLength={140} onChange={(e) => setDescription(e.target.value)} />
          <div className={styles.field}>
            <label htmlFor="group-category" className={styles.label}>
              {t('form.category')}
            </label>
            <select id="group-category" className={styles.select} value={categoryId || first?.category_id} onChange={(e) => setCategoryId(e.target.value)}>
              {list.map((c) => (
                <option key={c.id} value={c.id}>
                  {categoryName(t, c)}
                </option>
              ))}
            </select>
          </div>
          {ctx.partner && (
            <>
              <ChoiceGroup
                legend={t('form.paidBy')}
                options={ctx.people.map((p) => ({ value: p.id, label: p.id === ctx.me.id ? t('form.me') : p.name }))}
                value={paidBy}
                onChange={setPaidBy}
              />
              <ChoiceGroup
                legend={t('split.legend')}
                options={[
                  { value: 'keep', label: t('group.keepSplit') },
                  { value: 'equal', label: t('split.equal') },
                  { value: 'custom', label: t('split.custom') },
                ]}
                value={splitMode}
                onChange={setSplitMode}
              />
              {splitMode === 'custom' && <SplitEditor people={ctx.people} value={custom} onChange={setCustom} />}
            </>
          )}
          <div className={styles.footer}>
            <Button variant="quiet" onClick={() => setEditing(false)}>
              {tc('actions.cancel')}
            </Button>
            <Button variant="primary" loading={actions.updateGroup.isPending} onClick={save}>
              {t('composer.save')}
            </Button>
          </div>
        </div>
      ) : confirming ? (
        <div className={styles.form}>
          <p>{t('group.confirmDelete', { open: openCount, settled: group.settled })}</p>
          <div className={styles.footer}>
            <Button variant="quiet" onClick={() => setConfirming(false)}>
              {tc('actions.cancel')}
            </Button>
            <Button variant="danger" loading={actions.removeGroup.isPending} onClick={remove}>
              {t('group.delete')}
            </Button>
          </div>
        </div>
      ) : (
        <div className={styles.footer}>
          {openCount > 0 && first?.can_edit && (
            <>
              <Button variant="quiet" onClick={() => setConfirming(true)}>
                {t('group.delete')}
              </Button>
              <Button variant="primary" onClick={() => setEditing(true)}>
                {t('group.edit')}
              </Button>
            </>
          )}
          <DialogClose asChild>
            <Button variant="quiet">{tc('actions.close')}</Button>
          </DialogClose>
        </div>
      )}
    </div>
  )
}
