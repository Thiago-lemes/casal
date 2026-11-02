import { useTranslation } from 'react-i18next'
import { ChoiceGroup } from '@/design-system'
import styles from '../goals.module.css'

/** Quem pode ver uma meta pessoal: só o dono (padrão) ou o parceiro lê o progresso. Meta do casal não tem escolha. */
export function VisibilityChoice({ value, onChange }: { value: 'private' | 'view'; onChange: (v: 'private' | 'view') => void }) {
  const { t } = useTranslation('goals')
  return (
    <>
      <ChoiceGroup
        legend={t('visibility.legend')}
        options={[
          { value: 'private', label: t('visibility.private') },
          { value: 'view', label: t('visibility.view') },
        ]}
        value={value}
        onChange={onChange}
      />
      <p className={styles.hint}>{t(value === 'private' ? 'visibility.privateHint' : 'visibility.viewHint')}</p>
    </>
  )
}
