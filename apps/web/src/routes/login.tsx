import { createFileRoute } from '@tanstack/react-router'
import { useWebNavigate } from '../use-web-navigate'
import { LoginPage, safeRedirect } from '@0xunknownz/foundation-tenant-ui'

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? (search.redirect as string) : undefined,
  }),
  component: LoginRoute,
})

function LoginRoute() {
  const navigate = useWebNavigate()
  const { redirect } = Route.useSearch()
  return <LoginPage redirect={redirect} navigate={(path) => {
    // Foundation's login page selects the required MFA step; retain the product destination.
    const isMfaStep = path === '/mfa/verify' || path === '/mfa/enroll'
    const href = isMfaStep && redirect !== undefined
      ? path + '?redirect=' + encodeURIComponent(safeRedirect(redirect, '/'))
      : path
    navigate(href)
  }} />
}
