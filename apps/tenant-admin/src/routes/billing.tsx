import { useRouter } from '@tanstack/react-router'
import { BillingPage } from '@0xunknownz/foundation-tenant-ui'
import { resolveTenantAdminPath } from './members'

export function BillingRoute() {
  const router = useRouter()
  return (
    <BillingPage
      navigate={(path) => {
        if (resolveTenantAdminPath(path) === 'route') void router.navigate({ to: path })
        else window.location.assign(path)
      }}
    />
  )
}
