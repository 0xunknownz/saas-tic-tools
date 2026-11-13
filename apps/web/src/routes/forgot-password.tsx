import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { ForgotPasswordPage } from '@0xunknownz/foundation-tenant-ui'

export const Route = createFileRoute('/forgot-password')({ component: ForgotPasswordRoute })

function ForgotPasswordRoute() {
  const navigate = useNavigate()
  return <ForgotPasswordPage navigate={(path) => void navigate({ href: path })} />
}
