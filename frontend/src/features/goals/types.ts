/** O mínimo que a feature precisa saber de uma pessoa; quem compõe a tela entrega isso (as features não se importam). */
export type Person = { id: string; name: string }

export type Me = Person & { timezone: string }

/** O casal visto pela feature: quem sou eu, o parceiro (se já entrou) e as duas pessoas. */
export type Ctx = { me: Me; partner?: Person; people: Person[] }

export const nameOf = (ctx: Ctx, id: string | null | undefined) => ctx.people.find((p) => p.id === id)?.name ?? ''

/**
 * O que Finanças oferece à criação de uma meta de economia do orçamento: as categorias que têm limite.
 * `undefined` quando o módulo Finanças está desligado. Quem compõe a tela monta isto (Metas não importa Finanças).
 */
export type FinanceOptions = { categories: { id: string; name: string }[] }
