import { describe, expect, it } from 'vitest'
import { resolveWebPath } from '../src/cross-app'

describe('web cross-app navigation', () => {
  it('sends members to tenant-admin under /console/ with a full-page navigation', () => {
    expect(resolveWebPath('/members')).toEqual({ kind: 'page', href: '/console/members' })
    expect(resolveWebPath('/billing')).toEqual({ kind: 'page', href: '/console/billing' })
  })

  it('opens console and platform return destinations as full pages, preserving query and fragment', () => {
    for (const path of ['/console/members', '/console/billing?interval=year#plans', '/console/', '/admin/']) {
      expect(resolveWebPath(path)).toEqual({ kind: 'page', href: path })
    }
    expect(resolveWebPath('/members?filter=active#table')).toEqual({ kind: 'page', href: '/console/members?filter=active#table' })
    expect(resolveWebPath('/console-preview')).toEqual({ kind: 'route', href: '/console-preview' })
  })

  it('keeps web-owned entry pages in the router', () => {
    for (const path of ['/login', '/tenant/select', '/', '/invitations/accept']) {
      expect(resolveWebPath(path)).toEqual({ kind: 'route', href: path })
    }
  })
})
