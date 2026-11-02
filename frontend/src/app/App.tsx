import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { ApiError } from '@/shared/api/client'
import { AuthPage } from '@/features/auth/AuthPage'
import { HomePage } from '@/features/household/HomePage'
import { TodayPage } from '@/features/today/TodayPage'
import { RequireAuth, RequireHousehold } from './guards'
import { Shell } from './Shell'

// uma página por pedaço de código: o carregamento inicial não cresce a cada tela nova
const ProfileRoute = lazy(() => import('./ProfileRoute').then((m) => ({ default: m.ProfileRoute })))
const TasksRoute = lazy(() => import('./TasksRoute').then((m) => ({ default: m.TasksRoute })))

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // não insiste em erros do cliente (4xx); tenta de novo só falhas de rede ou do servidor
      retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
      refetchOnWindowFocus: true,
    },
  },
})

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<AuthPage mode="login" />} />
          <Route path="/register" element={<AuthPage mode="register" />} />
          <Route element={<RequireAuth />}>
            <Route element={<Shell />}>
              <Route
                path="/profile"
                element={
                  <Suspense fallback={null}>
                    <ProfileRoute />
                  </Suspense>
                }
              />
              <Route element={<RequireHousehold />}>
                <Route index element={<TodayPage />} />
                <Route path="/home" element={<HomePage />} />
                <Route
                  path="/tasks"
                  element={
                    <Suspense fallback={null}>
                      <TasksRoute />
                    </Suspense>
                  }
                />
                <Route path="*" element={<Navigate to="/" replace />} />
              </Route>
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
