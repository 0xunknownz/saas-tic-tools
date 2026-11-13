import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { MfaVerifyPage } from '@0xunknownz/foundation-tenant-ui'

export const Route = createFileRoute('/mfa/verify')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? (search.redirect as string) : undefined,
  }),
  component: MfaVerifyRoute,
})

function MfaVerifyRoute() {
  const navigate = useNavigate()
  const { redirect } = Route.useSearch()
  return <MfaVerifyPage redirect={redirect} navigate={(path) => void navigate({ href: path })} />
}
