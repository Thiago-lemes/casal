import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Card } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import type { Goal, Step } from '../api'
import { useStepActions } from '../hooks'
import { goalActions } from '../permissions'
import type { Ctx } from '../types'
import { StepEditor } from './StepEditor'
import { StepRow } from './StepRow'
import styles from '../goals.module.css'

/** Projeto: as etapas em ordem. Etapa sem tarefa é checklist; com tarefa, segue a tarefa. */
export function ProjectPanel({ goal, ctx }: { goal: Goal; ctx: Ctx }) {
  const { t } = useTranslation('goals')
  const { t: tc } = useTranslation('common')
  const actions = useStepActions(goal.id)
  const [editing, setEditing] = useState<Step | 'new'>()
  const editable = goalActions(goal).addStep
  const steps = [...(goal.steps ?? [])].sort((a, b) => a.position - b.position)
  const error = actions.done.error ?? actions.update.error
  const busy = actions.done.isPending || actions.update.isPending

  // sobe e desce pela posição do vizinho: funciona com a posição começando em 0 ou em 1
  function move(index: number, direction: -1 | 1) {
    const other = steps[index + direction]
    if (other) actions.update.mutate({ sid: steps[index].id, body: { position: other.position } })
  }

  return (
    <Card
      title={t('project.title')}
      aside={
        editable && (
          <Button variant="quiet" onClick={() => setEditing('new')}>
            {t('project.add')}
          </Button>
        )
      }
    >
      <div className={styles.stack}>
        {goal.sync_pending && (
          <p className={styles.hint} role="status">
            {t('project.syncPending')}
          </p>
        )}
        {steps.length === 0 ? (
          <p className={styles.hint}>{t('project.empty')}</p>
        ) : (
          <ul className={styles.steps}>
            {steps.map((s, i) => (
              <StepRow
                key={s.id}
                step={s}
                ctx={ctx}
                editable={editable}
                first={i === 0}
                last={i === steps.length - 1}
                busy={busy}
                onToggle={(done) => actions.done.mutate({ sid: s.id, done })}
                onEdit={() => setEditing(s)}
                onMove={(d) => move(i, d)}
              />
            ))}
          </ul>
        )}
        {error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, error)}
          </p>
        )}
      </div>
      {editing && <StepEditor goal={goal} ctx={ctx} step={editing === 'new' ? undefined : editing} onClose={() => setEditing(undefined)} />}
    </Card>
  )
}
