import { useState } from 'react'
import { useMe } from '@/features/auth/hooks'
import { FromShoppingDialog } from '@/features/finance/expenses/FromShoppingDialog'
import { useHousehold } from '@/features/household/hooks'
import { ShoppingPage } from '@/features/shopping/ShoppingPage'
import { useModuleEnabled } from '@/shared/modules/hooks'

/** Compõe Compras com Finanças: com o módulo ligado, o modo mercado oferece registrar o gasto da compra. */
export function ShoppingRoute() {
  const me = useMe().data
  const household = useHousehold().data
  const finance = useModuleEnabled('finance')
  const [registering, setRegistering] = useState(false)
  const people = household?.members ?? []

  return (
    <>
      <ShoppingPage onRegisterExpense={finance.on && me && household ? () => setRegistering(true) : undefined} />
      {registering && me && household && (
        <FromShoppingDialog
          ctx={{ me, people, partner: people.find((p) => p.id !== me.id) }}
          onClose={() => setRegistering(false)}
        />
      )}
    </>
  )
}
