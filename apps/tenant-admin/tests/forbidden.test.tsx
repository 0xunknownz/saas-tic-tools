import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { Root } from '../src/root'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

it('shows an actionable forbidden state for a real member session instead of a blank page', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
    accountId: 'member-acceptance', email: 'acceptance@example.test',
    tenant: { id: 'tenant-1', slug: 'acme', name: '验收机构', status: 'active' },
    role: 'member', permissions: ['tenant.members.read'],
  }), { status: 200, headers: { 'Content-Type': 'application/json' } })))
  render(<Root />)
  expect(await screen.findByRole('heading', { name: '没有管理权限' })).toBeTruthy()
  expect(screen.getByRole('link', { name: '返回工作台' }).getAttribute('href')).toBe('/')
  expect(screen.getByRole('link', { name: '切换租户' }).getAttribute('href')).toContain('/tenant/select?redirect=')
  expect(screen.queryByRole('navigation', { name: '租户后台' })).toBeNull()
})
