import { Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button, ButtonLink, Card } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useSetModule } from '@/shared/modules/hooks'
import { exportUrl } from '../api'
import { RecurringList } from '../recurring/RecurringList'
import { BudgetEditor } from '../summary/BudgetEditor'
import type { Ctx } from '../types'
import { CategoriesManager } from './CategoriesManager'
import { DivisionSettings } from './DivisionSettings'
import { IncomeField } from './IncomeField'
import styles from '../finance.module.css'

/** Tudo que se configura em Finanças, numa página só (abre da engrenagem). */
export function SettingsView({ ctx, onBack }: { ctx: Ctx; onBack: () => void }) {
  const { t } = useTranslation('finance')
  const { t: tc } = useTranslation('common')
  const module = useSetModule('finance')

  return (
    <div className={styles.stack}>
      <div>
        <Button variant="quiet" onClick={onBack}>
          {t('settings.back')}
        </Button>
      </div>

      {ctx.partner && <DivisionSettings ctx={ctx} />}
      <IncomeField />
      <RecurringList ctx={ctx} />
      <BudgetEditor />
      <CategoriesManager />

      <Card title={t('export.title')}>
        <p className={styles.hint}>{t('export.help')}</p>
        <div className={styles.segmented}>
          {/* link direto: o navegador cuida do arquivo */}
          <ButtonLink href={exportUrl('csv')} download>
            <Download size={18} aria-hidden="true" />
            {t('export.csv')}
          </ButtonLink>
          <ButtonLink href={exportUrl('json')} download>
            {t('export.json')}
          </ButtonLink>
        </div>
      </Card>

      <Card title={t('module.offTitle')}>
        <p className={styles.hint}>{t('module.offHelp')}</p>
        {module.error != null && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, module.error)}
          </p>
        )}
        <div>
          <Button loading={module.isPending} onClick={() => module.mutate(false)}>
            {t('module.disable')}
          </Button>
        </div>
      </Card>
    </div>
  )
}
