/** O mínimo que a feature precisa saber de uma pessoa; quem compõe a tela entrega isso (as features não se importam). */
export type Person = { id: string; name: string }

export type Me = Person & { timezone: string }
