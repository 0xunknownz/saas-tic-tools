import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { HomePage } from '@0xunknownz/foundation-tenant-ui'
import { resolveWebPath } from '../cross-app'

export const Route = createFileRoute('/')({ component: HomeRoute })

function HomeRoute() {
  const navigate = useNavigate()
  return (
    <HomePage
      navigate={(path) => {
        const target = resolveWebPath(path)
        if (target.kind === 'route') void navigate({ href: target.href })
        else window.location.assign(target.href)
      }}
    />
  )
}
