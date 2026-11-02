/** O que a rotina precisa saber de quem usa; a tela que compõe as features entrega isso. */
export type Me = { id: string; name: string; timezone: string }
export type Person = { id: string; name: string }

/** `me` ou `partner`: decide a cor que identifica de quem é o bloco. */
export const toneOf = (ownerId: string, meId: string): 'me' | 'partner' => (ownerId === meId ? 'me' : 'partner')
