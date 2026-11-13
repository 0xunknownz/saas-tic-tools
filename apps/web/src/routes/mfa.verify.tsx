import { createFileRoute } from '@tanstack/react-router'
import { useWebNavigate } from '../use-web-navigate'
import { MfaVerifyPage } from '@0xunknownz/foundation-tenant-ui'

export const Route = createFileRoute('/mfa/verify')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? (search.redirect as string) : undefined,
  }),
  component: MfaVerifyRoute,
})

function MfaVerifyRoute() {
  const navigate = useWebNavigate()
  const { redirect } = Route.useSearch()
  return <MfaVerifyPage redirect={redirect} navigate={navigate} />
}
