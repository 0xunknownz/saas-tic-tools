import { useRouter } from '@tanstack/react-router'
import { MembersPage } from '@0xunknownz/foundation-tenant-ui'

// Paths owned by tenant-admin stay in the router; paths owned by web (login, tenant selection)
// become a full-page navigation on the same host, so the tenant session cookie still applies.
export function resolveTenantAdminPath(path: string): 'route' | 'page' {
  return path === '/members' || path === '/billing' ? 'route' : 'page'
}

export function MembersRoute() {
  const router = useRouter()
  return (
    <MembersPage
      navigate={(path) => {
        if (resolveTenantAdminPath(path) === 'route') void router.navigate({ to: path })
        else window.location.assign(path)
      }}
    />
  )
}
