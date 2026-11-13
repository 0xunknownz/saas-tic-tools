import { describe, expect, it } from 'vitest'
import { decideSession, sessionEntryHref } from '../src/root'
import type { UseSessionResult } from '@0xunknownz/foundation-tenant-ui'

function state(overrides: Partial<UseSessionResult>): UseSessionResult {
  return { session: null, loading: false, error: null, refresh: async () => null, ...overrides }
}

const signedIn = (role: string | null, tenant: object | null) =>
  state({ session: { email: 'a@example.com', role, tenant } as never })

describe('tenant-admin root guard (design 4.1)', () => {
  it('retains the current management destination through login and tenant selection', () => {
    const location = { pathname: '/console/members', search: '?filter=active', hash: '#table' }
    for (const decision of ['login', 'select'] as const) {
      const href = new URL(sessionEntryHref(decision, location), 'https://example.test')
      expect(href.pathname).toBe(decision === 'login' ? '/login' : '/tenant/select')
      expect(href.searchParams.get('redirect')).toBe('/console/members?filter=active#table')
    }
  })

  it('keeps showing loading while the session request runs', () => {
    expect(decideSession(state({ loading: true }))).toBe('loading')
  })

  it('treats a failed request as an error, not as signed out', () => {
    expect(decideSession(state({ error: { code: 'NETWORK', status: 0 } as never }))).toBe('error')
  })

  it('sends an unauthenticated visitor to web login', () => {
    expect(decideSession(state({}))).toBe('login')
  })

  it('sends a signed-in user without a tenant to tenant selection', () => {
    expect(decideSession(signedIn(null, null))).toBe('select')
  })

  it('stops a member without redirecting to another app', () => {
    expect(decideSession(signedIn('member', { id: 't1', name: 'T', slug: 't' }))).toBe('forbidden')
  })

  it('renders the pages for a tenant administrator', () => {
    expect(decideSession(signedIn('tenant_admin', { id: 't1', name: 'T', slug: 't' }))).toBe('ok')
  })
})
