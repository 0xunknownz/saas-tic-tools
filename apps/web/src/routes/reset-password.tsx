import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { ResetPasswordPage } from '@0xunknownz/foundation-tenant-ui'

function stringParam(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined
}

export const Route = createFileRoute('/reset-password')({
  validateSearch: (search: Record<string, unknown>) => ({
    ['token']: stringParam(search.token),
  }),
  component: ResetPasswordRoute,
})

function ResetPasswordRoute() {
  const navigate = useNavigate()
  const { token } = Route.useSearch()
  return <ResetPasswordPage token={token ?? ''} navigate={(path) => void navigate({ href: path })} />
}
