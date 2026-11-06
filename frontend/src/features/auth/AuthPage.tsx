import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Navigate } from 'react-router-dom'
import { Button, Logo, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { APP_NAME } from '@/shared/config'
import { useLogin, useMe, useRegister, useSignupMode } from './hooks'
import styles from './AuthPage.module.css'

export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const { t } = useTranslation('auth')
  const { t: tc } = useTranslation('common')
  const me = useMe()
  const login = useLogin()
  const register = useRegister()
  const inviteOnly = useSignupMode().data?.signup_mode === 'invite'
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [signupCode, setSignupCode] = useState('')

  if (me.data) return <Navigate to="/" replace />

  const action = mode === 'login' ? login : register

  function submit(e: FormEvent) {
    e.preventDefault()
    if (mode === 'login') login.mutate({ email, password })
    else register.mutate({ email, name, password, signupCode: inviteOnly ? signupCode.trim() : undefined })
  }

  return (
    <main className={styles.page}>
      <div className={styles.band}>
        <Logo className={styles.brand} tone="onDark" title={APP_NAME} />
      </div>
      <form className={styles.form} onSubmit={submit}>
        <h1 className={styles.title}>{t(`${mode}.title`)}</h1>
        {mode === 'register' && (
          <TextField
            label={t('fields.name')}
            autoComplete="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        )}
        <TextField
          label={t('fields.email')}
          type="email"
          autoComplete="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
        <TextField
          label={t('fields.password')}
          type="password"
          autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
          hint={mode === 'register' ? t('fields.passwordHint') : undefined}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          minLength={mode === 'register' ? 10 : undefined}
          required
        />
        {mode === 'register' && inviteOnly && (
          <TextField
            label={t('fields.signupCode')}
            hint={t('fields.signupCodeHint')}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            value={signupCode}
            onChange={(e) => setSignupCode(e.target.value)}
            required
          />
        )}
        {action.isError && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, action.error)}
          </p>
        )}
        <Button type="submit" variant="primary" loading={action.isPending}>
          {t(`${mode}.submit`)}
        </Button>
        {mode === 'register' && <p className={styles.privacy}>{t('register.privacy')}</p>}
        <p className={styles.switch}>
          {t(`${mode}.switch`)} <Link to={mode === 'login' ? '/register' : '/login'}>{t(`${mode}.switchAction`)}</Link>
        </p>
      </form>
    </main>
  )
}
