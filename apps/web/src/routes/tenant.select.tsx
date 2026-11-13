import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { TenantSwitchPage } from '@0xunknownz/foundation-tenant-ui'

export const Route = createFileRoute('/tenant/select')({ component: TenantSelectRoute })

function TenantSelectRoute() {
  const navigate = useNavigate()
  return <TenantSwitchPage navigate={(path) => void navigate({ href: path })} />
}
