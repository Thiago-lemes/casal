import type { Balance } from '../api'

export type Standing =
  | { kind: 'even' }
  | { kind: 'iOwe'; to: string; cents: number }
  | { kind: 'owedToMe'; from: string; cents: number }

/** Quem deve a quem, na primeira pessoa. Sem `from_user` ou com valor zero: tudo em dia. */
export function standing(balance: Pick<Balance, 'from_user' | 'to_user' | 'amount_cents'>, meId: string): Standing {
  if (!balance.from_user || !balance.to_user || balance.amount_cents <= 0) return { kind: 'even' }
  return balance.from_user === meId
    ? { kind: 'iOwe', to: balance.to_user, cents: balance.amount_cents }
    : { kind: 'owedToMe', from: balance.from_user, cents: balance.amount_cents }
}
