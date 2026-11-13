// Cross-app navigation (design 4.1): pages now live in tenant-admin under /console/ on the same host.
// Paths owned by web stay in the router; members and billing become full-page navigations.
import { safeRedirect } from '@0xunknownz/foundation-tenant-ui'

const navigationOrigin = 'https://web-navigation.invalid'

// Recheck the serialized path: normalizing /console/..//host can produce //host.
function internalPath(path: string): string {
  if (!path.startsWith('/') || path.startsWith('//')) return '/'
  try {
    const url = new URL(path, navigationOrigin)
    if (url.origin !== navigationOrigin || url.pathname.startsWith('//')) return '/'
    return url.pathname + url.search + url.hash
  } catch {
    return '/'
  }
}

export function safeReturnPath(value: unknown): string {
  return internalPath(safeRedirect(value, '/'))
}

export interface Target {
  kind: 'route' | 'page'
  href: string
}

export function resolveWebPath(path: string): Target {
  // Generated MFA URLs can contain a long encoded return target; only the original
  // return value uses Foundation's length/encoding policy, while every navigation checks origin.
  const href = internalPath(path)
  const pathname = href.split(/[?#]/, 1)[0]
  if (pathname === '/members' || pathname === '/billing') return { kind: 'page', href: '/console' + href }
  if (pathname === '/console' || pathname.startsWith('/console/') || pathname === '/admin' || pathname.startsWith('/admin/')) {
    return { kind: 'page', href }
  }
  return { kind: 'route', href }
}
