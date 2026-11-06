import { Navigate, Route, Routes } from 'react-router-dom'
import { AdminLogin } from './AdminLogin'
import { AdminShell, RequireAdmin } from './AdminShell'
import { AuditPage } from './AuditPage'
import { CodesPage } from './CodesPage'
import { FeedbackPage } from './FeedbackPage'
import { HouseholdPage, HouseholdsPage, UserPage } from './HouseholdsPage'

/** Rotas do painel de suporte (/admin/*). Carregado sob demanda: quem não é admin não baixa este código. */
export function AdminApp() {
  return (
    <Routes>
      <Route path="login" element={<AdminLogin />} />
      <Route element={<RequireAdmin />}>
        <Route element={<AdminShell />}>
          <Route index element={<Navigate to="casais" replace />} />
          <Route path="casais" element={<HouseholdsPage />} />
          <Route path="casais/:id" element={<HouseholdPage />} />
          <Route path="pessoas/:id" element={<UserPage />} />
          <Route path="convites" element={<CodesPage />} />
          <Route path="feedback" element={<FeedbackPage />} />
          <Route path="auditoria" element={<AuditPage />} />
          <Route path="*" element={<Navigate to="/admin" replace />} />
        </Route>
      </Route>
    </Routes>
  )
}
