import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { LoginPage } from '@0xunknownz/foundation-tenant-ui'

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? (search.redirect as string) : undefined,
  }),
  component: LoginRoute,
})

function LoginRoute() {
  const navigate = useNavigate()
  const { redirect } = Route.useSearch()
  return <LoginPage redirect={redirect} navigate={(path) => void navigate({ href: path })} />
}
