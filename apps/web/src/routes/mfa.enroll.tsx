import { createFileRoute } from '@tanstack/react-router'
import { useWebNavigate } from '../use-web-navigate'
import { MfaEnrollPage } from '@0xunknownz/foundation-tenant-ui'

export const Route = createFileRoute('/mfa/enroll')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? (search.redirect as string) : undefined,
  }),
  component: MfaEnrollRoute,
})

function MfaEnrollRoute() {
  const navigate = useWebNavigate()
  const { redirect } = Route.useSearch()
  return <MfaEnrollPage redirect={redirect} navigate={navigate} />
}
