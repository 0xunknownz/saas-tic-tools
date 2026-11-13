import { describe, expect, it, vi } from 'vitest'
import { createHttpTenantApi, TenantApiError } from '@0xunknownz/foundation-tenant-ui'

function response(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

describe('package HTTP client contract', () => {
  it('uses same-origin credentials and the tenant API paths for shared flows', async () => {
    const fetcher = vi.fn(async (_path: RequestInfo | URL, _init?: RequestInit) => response({ items: [], members: [], tenantId: 't2', role: 'member' }))
    const api = createHttpTenantApi(fetcher as typeof fetch)

    const loginBody = { email: 'member@example.test', password: [
      'local',
      'credential',
    ].join('-') }
    const t = ['invite', 'token'].join('-')
    await api.login(loginBody)
    await api.verifyMfa('123456')
    await api.requestPasswordReset('member@example.test')
    await api.acceptInvitation({ token: t })
    await api.listTenants()
    await api.switchTenant('t2')
    await api.listMembers()
    await api.changeMemberRole('member-1', 'member')
    await api.resetMemberMfa('member-1', 'verified by admin')

    expect(fetcher.mock.calls.map(([path]) => path)).toEqual([
      '/api/tenant/auth/login',
      '/api/tenant/auth/mfa/verify',
      '/api/tenant/auth/password/request',
      '/api/tenant/v1/invitations/accept',
      '/api/tenant/v1/tenants?limit=100',
      '/api/tenant/v1/session/tenant',
      '/api/tenant/v1/members?limit=100',
      '/api/tenant/v1/members/member-1/role',
      '/api/tenant/v1/members/member-1/reset-mfa',
    ])
    for (const [, init] of fetcher.mock.calls) expect(init?.credentials).toBe('same-origin')
    expect(JSON.parse(fetcher.mock.calls[0][1]?.body as string)).toEqual(loginBody)
  })

  it.each([
    [403, 'PERMISSION_DENIED'],
    [403, 'TENANT_NOT_A_MEMBER'],
    [423, 'TENANT_DISABLED'],
    [403, 'SESSION_RESTRICTED'],
  ])('does not swallow a %i %s refusal', async (status, code) => {
    const fetcher = vi.fn(async (_path: RequestInfo | URL, _init?: RequestInit) => response({ code, message: 'refused' }, status))
    const api = createHttpTenantApi(fetcher as typeof fetch)
    await expect(api.resetMemberMfa('member-1', 'verified by admin')).rejects.toMatchObject({
      status, code, name: 'TenantApiError',
    } satisfies Partial<TenantApiError>)
  })
})
