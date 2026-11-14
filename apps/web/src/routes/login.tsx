import { createFileRoute } from '@tanstack/react-router'
import { useWebNavigate } from '../use-web-navigate'
import { TicLoginPage } from '../features/auth/TicLoginPage'
import { safeReturnPath } from '../cross-app'

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? (search.redirect as string) : undefined,
  }),
  component: LoginRoute,
})

function LoginRoute() {
  const navigate = useWebNavigate()
  const { redirect } = Route.useSearch()
  return <TicLoginPage redirect={redirect} navigate={(path) => {
    // Foundation's login page selects the required MFA step; retain the product destination.
    const isMfaStep = path === '/mfa/verify' || path === '/mfa/enroll'
    const href = isMfaStep && redirect !== undefined
      ? path + '?redirect=' + encodeURIComponent(safeReturnPath(redirect))
      : path
    navigate(href)
  }} />
}
