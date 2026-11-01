import { useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Navigate } from 'react-router-dom'
import { Button, TextField } from '@/design-system'
import { errorMessage } from '@/shared/api/errorMessage'
import { APP_NAME } from '@/shared/config'
import { useLogin, useMe, useRegister } from './hooks'
import styles from './AuthPage.module.css'

export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const { t } = useTranslation('auth')
  const { t: tc } = useTranslation('common')
  const me = useMe()
  const login = useLogin()
  const register = useRegister()
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')

  if (me.data) return <Navigate to="/" replace />

  const action = mode === 'login' ? login : register

  function submit(e: FormEvent) {
    e.preventDefault()
    if (mode === 'login') login.mutate({ email, password })
    else register.mutate({ email, name, password })
  }

  return (
    <main className={styles.page}>
      <div className={styles.band}>
        <p className={styles.brand}>{APP_NAME}</p>
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
        {action.isError && (
          <p className={styles.error} role="alert">
            {errorMessage(tc, action.error)}
          </p>
        )}
        <Button type="submit" variant="primary" loading={action.isPending}>
          {t(`${mode}.submit`)}
        </Button>
        <p className={styles.switch}>
          {t(`${mode}.switch`)} <Link to={mode === 'login' ? '/register' : '/login'}>{t(`${mode}.switchAction`)}</Link>
        </p>
      </form>
    </main>
  )
}
