import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, EmptyState, Skeleton, Tabs } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { routineApi } from './api'
import type { Block, BlockInput, Occurrence } from './api'
import { BlockEditor } from './BlockEditor'
import { addDays, dayKeyIn, formatDay, weekDays, weekStart } from './dates'
import { DayView } from './DayView'
import { FreeView } from './FreeView'
import type { MinOption } from './FreeView'
import { useBlocks, useFree, useNow, useOccurrences, useRoutineActions } from './hooks'
import { NowCard } from './NowCard'
import { OccurrenceDialog } from './OccurrenceDialog'
import type { Me, Person } from './types'
import { WeekView } from './WeekView'
import styles from './RoutinePage.module.css'

type View = 'day' | 'week' | 'free'
const VIEWS: View[] = ['day', 'week', 'free']

/** Pode alterar o bloco: o dono, ou qualquer um dos dois se for do casal. Bloco "ocupado" do parceiro nunca. */
export const canEditBlock = (b: Block | undefined, meId: string) =>
  !!b && !b.busy && (b.owner_id === meId || b.visibility === 'household')

/** `people` traz o casal inteiro (inclui quem está logado); a tela que compõe as features entrega isso. */
export function RoutinePage({ me, people }: { me: Me; people: Person[] }) {
  const { t, i18n } = useTranslation('routine')
  const { t: tc } = useTranslation('common')
  const today = useMemo(() => dayKeyIn(new Date(), me.timezone), [me.timezone])
  const [view, setView] = useState<View>('day')
  const [day, setDay] = useState(today)
  const [min, setMin] = useState<MinOption>(60)
  const [selected, setSelected] = useState<{ occ: Occurrence; day: string } | null>(null)
  const [editing, setEditing] = useState<Block | 'new' | null>(null)
  const [notice, setNotice] = useState<string>()

  const actions = useRoutineActions()
  const blocks = useBlocks()
  const now = useNow()
  const partner = people.find((p) => p.id !== me.id)

  const from = view === 'week' ? weekStart(day) : day
  const to = view === 'week' ? addDays(weekStart(day), 6) : day
  const occurrences = useOccurrences(from, to)
  const free = useFree(day, min, view === 'free')

  const step = view === 'week' ? 7 : 1
  const label =
    view === 'week'
      ? `${formatDay(weekDays(day)[0], i18n.language, { day: 'numeric', month: 'short' })} – ${formatDay(weekDays(day)[6], i18n.language, { day: 'numeric', month: 'short' })}`
      : formatDay(day, i18n.language)

  const block = (id: string) => blocks.data?.find((b) => b.id === id)
  const editingBlock = editing === 'new' ? null : editing

  function afterSave(saved: Block) {
    setEditing(null)
    const others = (saved.overlaps ?? []).map((id) => block(id)?.title).filter(Boolean)
    setNotice(others.length > 0 ? t('notice.overlaps', { names: others.join(', ') }) : undefined)
  }

  function save(body: BlockInput, version?: number) {
    setNotice(undefined)
    if (editingBlock && version !== undefined) {
      actions.update.mutate({ id: editingBlock.id, body: { ...body, version } }, { onSuccess: afterSave })
    } else {
      actions.create.mutate(body, { onSuccess: afterSave })
    }
  }

  const loading = occurrences.isPending

  return (
    <div className={styles.page}>
      <header className={styles.hero}>
        <div>
          <h1 className={styles.title}>{t('title')}</h1>
          <p className={styles.range}>{label}</p>
        </div>
        <Button variant="primary" className={styles.cta} onClick={() => setEditing('new')}>
          {t('new')}
        </Button>
      </header>

      <div className={styles.layout}>
        <div className={styles.main}>
          <Tabs label={t('views.label')} value={view} onChange={setView} tabs={VIEWS.map((v) => ({ value: v, label: t(`views.${v}`) }))}>
            <div className={styles.nav}>
              <Button variant="secondary" onClick={() => setDay(addDays(day, -step))} aria-label={t('nav.prev')}>
                <ChevronLeft size={20} aria-hidden="true" />
              </Button>
              <Button variant="secondary" onClick={() => setDay(today)} disabled={weekDays(day).includes(today) && view === 'week' ? false : day === today}>
                {t('nav.today')}
              </Button>
              <Button variant="secondary" onClick={() => setDay(addDays(day, step))} aria-label={t('nav.next')}>
                <ChevronRight size={20} aria-hidden="true" />
              </Button>
              {view !== 'week' && <p className={styles.dayLabel}>{label}</p>}
            </div>

            {notice && (
              <p className={styles.notice} role="status">
                {notice}
              </p>
            )}

            <div aria-live="polite">
              {view === 'free' ? (
                free.isPending ? (
                  <Skeleton height="6rem" />
                ) : free.isError ? (
                  <EmptyState title={t('loadError')} body={errorMessage(tc, free.error)} action={<Button onClick={() => void free.refetch()}>{tc('actions.tryAgain')}</Button>} />
                ) : (
                  <FreeView windows={free.data ?? []} timezone={me.timezone} min={min} onMinChange={setMin} />
                )
              ) : loading ? (
                <Skeleton height="14rem" />
              ) : occurrences.isError ? (
                <EmptyState title={t('loadError')} body={errorMessage(tc, occurrences.error)} action={<Button onClick={() => void occurrences.refetch()}>{tc('actions.tryAgain')}</Button>} />
              ) : view === 'day' ? (
                (occurrences.data ?? []).length === 0 && blocks.data?.length === 0 ? (
                  <EmptyState
                    title={t('empty.title')}
                    body={t('empty.body')}
                    action={
                      <Button variant="primary" onClick={() => setEditing('new')}>
                        {t('new')}
                      </Button>
                    }
                  />
                ) : (
                  <DayView day={day} occurrences={occurrences.data ?? []} me={me} partner={partner} onSelect={(occ) => setSelected({ occ, day })} />
                )
              ) : (
                <WeekView
                  day={day}
                  today={today}
                  occurrences={occurrences.data ?? []}
                  me={me}
                  onSelect={(occ, d) => setSelected({ occ, day: d })}
                  onOpenDay={(d) => {
                    setDay(d)
                    setView('day')
                  }}
                />
              )}
            </div>
          </Tabs>
        </div>

        <aside className={styles.aside}>
          <NowCard people={now.data} pending={now.isPending} meId={me.id} timezone={me.timezone} />
        </aside>
      </div>

      {selected && (
        <OccurrenceDialog
          key={`${selected.occ.block_id}-${selected.day}`}
          open
          onOpenChange={(o) => !o && setSelected(null)}
          occurrence={selected.occ}
          day={selected.day}
          timezone={me.timezone}
          canEdit={canEditBlock(block(selected.occ.block_id), me.id)}
          pending={actions.setException.isPending || actions.clearException.isPending}
          error={actions.setException.error ?? actions.clearException.error}
          onSkip={() =>
            actions.setException.mutate({ id: selected.occ.block_id, date: selected.day, body: { skip: true } }, { onSuccess: () => setSelected(null) })
          }
          onChangeTime={(start, end) =>
            actions.setException.mutate({ id: selected.occ.block_id, date: selected.day, body: { start, end } }, { onSuccess: () => setSelected(null) })
          }
          onClear={() => actions.clearException.mutate({ id: selected.occ.block_id, date: selected.day }, { onSuccess: () => setSelected(null) })}
          onEditBlock={() => {
            const b = block(selected.occ.block_id)
            setSelected(null)
            if (b) setEditing(b)
          }}
        />
      )}

      {editing !== null && (
        <BlockEditor
          key={editingBlock?.id ?? 'new'}
          open
          onOpenChange={(o) => !o && setEditing(null)}
          block={editingBlock}
          today={today}
          isOwner={!editingBlock || editingBlock.owner_id === me.id}
          readOnly={!!editingBlock && !canEditBlock(editingBlock, me.id)}
          pending={actions.create.isPending || actions.update.isPending}
          error={editingBlock ? actions.update.error : actions.create.error}
          onSave={save}
          onDelete={editingBlock ? () => actions.remove.mutate(editingBlock.id, { onSuccess: () => setEditing(null) }) : undefined}
          onLoadCurrent={editingBlock ? () => routineApi.get(editingBlock.id) : undefined}
        />
      )}
    </div>
  )
}
