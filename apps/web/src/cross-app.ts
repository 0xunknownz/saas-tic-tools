// Cross-app navigation (design 4.1): pages now live in tenant-admin under /console/ on the same host.
// Paths owned by web stay in the router; members and billing become full-page navigations.
export interface Target {
  kind: 'route' | 'page'
  href: string
}

export function resolveWebPath(path: string): Target {
  const pathname = path.split(/[?#]/, 1)[0]
  if (pathname === '/members' || pathname === '/billing') return { kind: 'page', href: '/console' + path }
  if (pathname === '/console' || pathname.startsWith('/console/') || pathname === '/admin' || pathname.startsWith('/admin/')) {
    return { kind: 'page', href: path }
  }
  return { kind: 'route', href: path }
}
