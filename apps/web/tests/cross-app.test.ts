import { describe, expect, it } from 'vitest'
import { resolveWebPath, safeReturnPath } from '../src/cross-app'

describe('web cross-app navigation', () => {
  it('retains strict return-address checks for encoded traversal, malformed values and schemes', () => {
    for (const value of ['javascript:alert(1)', 'data:text/html,unsafe', '/%2F%2Fevil.example', '/%252F%252Fevil.example', '/console/%2e%2e//evil.example', '/%zz', '/ok'+String.fromCharCode(10)+'unsafe', '/'+String.fromCharCode(92)+'evil.example', null, undefined]) {
      expect(safeReturnPath(value)).toBe('/')
    }
  })

  it('preserves opaque token encoding, query and fragment for valid same-origin returns', () => {
    const target = '/invitations/accept?token=example%2Fvalue%2Bdata#details'
    expect(safeReturnPath(target)).toBe(target)
    expect(resolveWebPath('/console/members?token=example%2Fvalue%2Bdata#details')).toEqual({ kind: 'page', href: '/console/members?token=example%2Fvalue%2Bdata#details' })
  })

  it('rejects normalized protocol-relative and other unsafe destinations at the navigation boundary', () => {
    for (const path of ['/console/..//evil.example/landing', '/projects/../..//evil.example', '//evil.example/landing', 'https://evil.example/landing']) {
      expect(resolveWebPath(path)).toEqual({ kind: 'route', href: '/' })
    }
    expect(resolveWebPath('/console/../console/members?filter=active#table')).toEqual({ kind: 'page', href: '/console/members?filter=active#table' })
  })

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
