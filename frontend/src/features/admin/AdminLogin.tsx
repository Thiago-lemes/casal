import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate } from 'react-router-dom'
import { Button, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { useAdminLogin, useAdminMe } from './hooks'
import styles from './Admin.module.css'

/** Login do painel: e-mail, senha e o código de 6 dígitos do aplicativo autenticador (2FA obrigatório). */
export function AdminLogin() {
  const { t } = useTranslation('admin')
  const { t: tc } = useTranslation('common')
  const me = useAdminMe()
  const login = useAdminLogin()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')

  if (me.data) return <Navigate to="/admin" replace />

  function submit(e: FormEvent) {
    e.preventDefault()
    login.mutate({ email, password, code: code.trim() })
  }

  return (
    <form className={styles.login} onSubmit={submit}>
      <h1>{t('login.title')}</h1>
      <TextField label={t('login.email')} type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} required />
      <TextField
        label={t('login.password')}
        type="password"
        autoComplete="current-password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
      />
      <TextField
        label={t('login.code')}
        hint={t('login.codeHint')}
        inputMode="numeric"
        autoComplete="one-time-code"
        pattern="[0-9]{6}"
        maxLength={6}
        value={code}
        onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
        required
      />
      {login.isError && (
        <p className={styles.error} role="alert">
          {errorMessage(tc, login.error)}
        </p>
      )}
      <Button type="submit" variant="primary" loading={login.isPending}>
        {t('login.submit')}
      </Button>
    </form>
  )
}
