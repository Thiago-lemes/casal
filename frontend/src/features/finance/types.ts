/** O mínimo que a feature precisa saber de uma pessoa; quem compõe a tela entrega isso (as features não se importam). */
export type Person = { id: string; name: string }

export type Me = Person & { timezone: string }

/** O casal visto pela feature: quem sou eu, quem é o parceiro (se já entrou) e as duas pessoas. */
export type Ctx = { me: Me; partner?: Person; people: Person[] }

export const nameOf = (ctx: Ctx, id: string) => ctx.people.find((p) => p.id === id)?.name ?? ''
