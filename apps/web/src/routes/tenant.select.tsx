import { createFileRoute } from '@tanstack/react-router'
import { TicTenantSelectPage } from '../features/auth/TicTenantSelectPage'
import { useWebNavigate } from '../use-web-navigate'

export const Route = createFileRoute('/tenant/select')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
  }),
  component: TenantSelectRoute,
})

function TenantSelectRoute() {
  const navigate = useWebNavigate()
  const { redirect } = Route.useSearch()
  return <TicTenantSelectPage navigate={navigate} redirect={redirect} />
}
