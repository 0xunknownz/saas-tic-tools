import { useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { resolveWebPath } from './cross-app'

/** Authentication can finish in another app sharing this origin. */
export function useWebNavigate() {
  const navigate = useNavigate()
  return useCallback((path: string) => {
    const target = resolveWebPath(path)
    if (target.kind === 'page') window.location.assign(target.href)
    else void navigate({ href: target.href })
  }, [navigate])
}
