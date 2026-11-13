import { createFileRoute } from '@tanstack/react-router'
import { TenantSwitchPage, safeRedirect } from '@0xunknownz/foundation-tenant-ui'
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
    if (redirect !== undefined && path === '/') navigate(safeRedirect(redirect, '/'))
    else if (redirect !== undefined && path === '/login') {
      navigate('/login?redirect=' + encodeURIComponent(safeRedirect(redirect, '/')))
    } else navigate(path)
  }} />
}
