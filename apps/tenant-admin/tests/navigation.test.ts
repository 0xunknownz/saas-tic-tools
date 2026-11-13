import { describe, expect, it } from 'vitest'
import { resolveTenantAdminPath } from '../src/routes/members'

describe('tenant-admin cross-app navigation', () => {
  it('routes pages owned by tenant-admin', () => {
    expect(resolveTenantAdminPath('/members')).toBe('route')
    expect(resolveTenantAdminPath('/billing')).toBe('route')
  })

  it('leaves login, tenant selection and the front end to a full-page navigation', () => {
    for (const path of ['/login', '/tenant/select', '/']) {
      expect(resolveTenantAdminPath(path)).toBe('page')
    }
  })
})
