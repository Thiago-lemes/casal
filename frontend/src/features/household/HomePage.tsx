import { BellRing, Check, Copy, NotebookText, ShoppingCart } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Avatar, Button, Card } from '@/design-system'
import { useMe } from '@/features/auth/hooks'
import { errorMessage } from '@/shared/api/errorMessage'
import { useModuleEnabled, useSetModule } from '@/shared/modules/hooks'
import { useCreateInvite, useHousehold } from './hooks'
import styles from './HomePage.module.css'

/** Os módulos da casa; cada um é uma página própria. Os opcionais só entram aqui quando o casal os ativa. */
const MODULES: { to: string; key: string; icon: LucideIcon }[] = [
  { to: '/shopping', key: 'shopping', icon: ShoppingCart },
  { to: '/reminders', key: 'reminders', icon: BellRing },
  { to: '/notes', key: 'notes', icon: NotebookText },
]

/** "Casa": quem faz parte do casal e o convite para a segunda pessoa. */
export function HomePage() {
  const { t, i18n } = useTranslation('household')
  const { t: tc } = useTranslation('common')
  const me = useMe().data
  const household = useHousehold().data
  const invite = useCreateInvite()
  const finance = useModuleEnabled('finance', Boolean(household))
  const setFinance = useSetModule('finance')
  const [copied, setCopied] = useState(false)

  if (!me || !household) return null
  const partner = household.members.find((m) => m.id !== me.id)

  async function copy(token: string) {
    try {
      await navigator.clipboard.writeText(token)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2500)
    } catch {
      /* sem permissão de área de transferência: o código continua visível para copiar à mão */
    }
  }

  const expires =
    invite.data &&
    new Intl.DateTimeFormat(i18n.language, { day: 'numeric', month: 'long' }).format(new Date(invite.data.expires_at))

  return (
    <div className={styles.page}>
      <header className={styles.hero}>
        <h1 className={styles.title}>{household.members.map((m) => m.name).join(' & ')}</h1>
        <p className={styles.status}>{partner ? t('status.complete') : t('status.waiting')}</p>
      </header>

      <Card title={t('modules.title')} className={styles.modules}>
        <ul className={styles.moduleList}>
          {MODULES.map(({ to, key, icon: Icon }) => (
            <li key={key}>
              <Link to={to} className={styles.module}>
                <Icon size={22} aria-hidden="true" />
                <span>
                  <span className={styles.moduleName}>{t(`modules.${key}.name`)}</span>
                  <span className={styles.moduleHint}>{t(`modules.${key}.hint`)}</span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
        {/* módulo opcional desligado: um convite, nunca um erro; dá para desligar sem perder nada */}
        {finance.isSuccess && !finance.on && (
          <div className={styles.optional}>
            <p className={styles.moduleName}>{t('modules.finance.offTitle')}</p>
            <p className={styles.moduleHint}>{t('modules.finance.offHint')}</p>
            <Button loading={setFinance.isPending} onClick={() => setFinance.mutate(true)}>
              {t('modules.finance.enable')}
            </Button>
            {setFinance.error != null && (
              <p className={styles.error} role="alert">
                {errorMessage(tc, setFinance.error)}
              </p>
            )}
          </div>
        )}
      </Card>

      <Card title={t('members.title')} className={styles.members}>
        <ul className={styles.memberList}>
          {household.members.map((m) => {
            const mine = m.id === me.id
            return (
              <li key={m.id} className={styles.member} data-mine={mine || undefined}>
                <Avatar name={m.name} className={styles.avatar} />
                <div>
                  <p className={styles.memberName}>{mine ? t('members.you', { name: m.name }) : m.name}</p>
                  <p className={styles.memberSince}>
                    {t('members.since', {
                      date: new Intl.DateTimeFormat(i18n.language, { day: 'numeric', month: 'long', year: 'numeric' }).format(
                        new Date(m.joined_at),
                      ),
                    })}
                  </p>
                </div>
              </li>
            )
          })}
        </ul>
      </Card>

      {!partner && (
        <Card title={t('invite.title')} className={styles.invite}>
          <p className={styles.lead}>{t('invite.lead')}</p>
          {!invite.data && (
            <Button variant="primary" loading={invite.isPending} onClick={() => invite.mutate()}>
              {t('invite.action')}
            </Button>
          )}
          {invite.data && (
            <>
              <code className={styles.token}>{invite.data.token}</code>
              <p className={styles.expires}>{t('invite.expires', { date: expires })}</p>
              <Button onClick={() => void copy(invite.data.token)}>
                {copied ? <Check size={18} aria-hidden="true" /> : <Copy size={18} aria-hidden="true" />}
                {copied ? t('invite.copied') : t('invite.copy')}
              </Button>
            </>
          )}
          {invite.isError && (
            <p className={styles.error} role="alert">
              {errorMessage(tc, invite.error)}
            </p>
          )}
        </Card>
      )}
    </div>
  )
}
