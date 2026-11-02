import * as Radix from '@radix-ui/react-dialog'
import type { ReactNode } from 'react'
import styles from './Dialog.module.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  children: ReactNode
}

/** Folha que sobe de baixo no celular e vira diálogo central no desktop. O foco volta ao botão que a abriu (Radix). */
export function Dialog({ open, onOpenChange, title, children }: Props) {
  return (
    <Radix.Root open={open} onOpenChange={onOpenChange}>
      <Radix.Portal>
        <Radix.Overlay className={styles.overlay} />
        <Radix.Content className={styles.sheet} aria-describedby={undefined}>
          <Radix.Title className={styles.title}>{title}</Radix.Title>
          {children}
        </Radix.Content>
      </Radix.Portal>
    </Radix.Root>
  )
}

/** Envolva o botão que fecha o diálogo. */
export const DialogClose = Radix.Close
