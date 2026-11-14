import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { TicForgotPasswordPage } from '../features/auth/TicForgotPasswordPage'

export const Route = createFileRoute('/forgot-password')({ component: ForgotPasswordRoute })

function ForgotPasswordRoute() {
  const navigate = useNavigate()
  return <TicForgotPasswordPage navigate={(path) => void navigate({ href: path })} />
}
