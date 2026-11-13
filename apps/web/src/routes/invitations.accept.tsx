import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { AcceptInvitationPage } from '@0xunknownz/foundation-tenant-ui'

function stringParam(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined
}

export const Route = createFileRoute('/invitations/accept')({
  validateSearch: (search: Record<string, unknown>) => ({
    ['token']: stringParam(search.token),
  }),
  component: AcceptInvitationRoute,
})

function AcceptInvitationRoute() {
  const navigate = useNavigate()
  const { token } = Route.useSearch()
  return <AcceptInvitationPage token={token ?? ''} navigate={(path) => void navigate({ href: path })} />
}
