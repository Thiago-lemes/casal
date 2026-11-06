import styles from './LoadingScreen.module.css'

type Props = {
  /** Texto para leitores de tela (a animação em si é decorativa). */
  label: string
}

/** Tela de carregamento com os "o" da Zoog pulando. Há um GIF por tema; o CSS mostra o do tema ativo. */
export function LoadingScreen({ label }: Props) {
  return (
    <div className={styles.screen} role="status" aria-live="polite">
      <img className={styles.light} src="/loading/zoog-loading-claro.gif" alt="" width={240} height={150} />
      <img className={styles.dark} src="/loading/zoog-loading-escuro.gif" alt="" width={240} height={150} />
      <span className={styles.label}>{label}</span>
    </div>
  )
}
