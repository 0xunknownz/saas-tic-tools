import { createFileRoute } from '@tanstack/react-router'
import { TenantSwitchPage } from '@0xunknownz/foundation-tenant-ui'
import { safeReturnPath } from '../cross-app'
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
  return <TenantSwitchPage navigate={(path) => {
    if (redirect !== undefined && path === '/') navigate(safeReturnPath(redirect))
    else if (redirect !== undefined && path === '/login') {
      navigate('/login?redirect=' + encodeURIComponent(safeReturnPath(redirect)))
    } else navigate(path)
  }} />
}
