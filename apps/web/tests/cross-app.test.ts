import { describe, expect, it } from 'vitest'
import { resolveWebPath } from '../src/cross-app'

describe('web cross-app navigation', () => {
  it('sends members to tenant-admin under /console/ with a full-page navigation', () => {
    expect(resolveWebPath('/members')).toEqual({ kind: 'page', href: '/console/members' })
    expect(resolveWebPath('/billing')).toEqual({ kind: 'page', href: '/console/billing' })
  })

  it('keeps web-owned entry pages in the router', () => {
    for (const path of ['/login', '/tenant/select', '/', '/invitations/accept']) {
      expect(resolveWebPath(path)).toEqual({ kind: 'route', href: path })
    }
  })
})
