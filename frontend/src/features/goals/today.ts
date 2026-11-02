import type { Goal } from './api'
import { allowsDay } from './weekdays'

export type TodayGoals = { habits: Goal[]; shared: Goal[] }

/**
 * O que a pessoa ainda pode marcar hoje: os hábitos dela e as metas do casal, ativos e sem marcação de hoje.
 * Hábito fora dos dias combinados, ou hábito e meta do casal com a meta do período já batida, não entram (o mesmo critério do lembrete:
 * não se insiste em quem já cumpriu). As metas do parceiro (`view`) ficam na lista, nunca aqui.
 */
export function goalsForToday(goals: Goal[], meId: string, today: string): TodayGoals {
  const open = goals.filter((g) => g.status === 'active' && g.access === 'full' && g.progress.marked_today === false)
  return {
    habits: open.filter(
      (g) =>
        g.kind === 'habit' &&
        g.owner_id === meId &&
        allowsDay(g.habit?.weekdays ?? 0, today) &&
        g.progress.done < g.progress.total,
    ),
    shared: open.filter((g) => g.kind === 'shared' && g.scope === 'household' && g.progress.done < g.progress.total),
  }
}
