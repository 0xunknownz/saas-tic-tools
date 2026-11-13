import { fireEvent, screen, waitFor } from '@testing-library/react'
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router'
import { describe, expect, it, vi } from 'vitest'
import { Route as LoginRoute } from '../src/routes/login'
import { Route as VerifyRoute } from '../src/routes/mfa.verify'
import { Route as EnrollRoute } from '../src/routes/mfa.enroll'
import { Route as SelectRoute } from '../src/routes/tenant.select'
import { fieldByName, mockApi, renderWithProviders, TEST_PW } from './helpers'

function renderLogin(level: 'mfa_pending' | 'setup', target: string) {
  const root = createRootRoute({ component: Outlet })
  const home = createRoute({ getParentRoute: () => root, path: '/', component: () => <h1>工作台</h1> })
  const project = createRoute({ getParentRoute: () => root, path: '/projects/project-001', component: () => <h1>目标项目</h1> })
  const routeTree = root.addChildren([
    LoginRoute.update({ getParentRoute: () => root, id: '/login', path: '/login' } as never),
    VerifyRoute.update({ getParentRoute: () => root, id: '/mfa/verify', path: '/mfa/verify' } as never),
    EnrollRoute.update({ getParentRoute: () => root, id: '/mfa/enroll', path: '/mfa/enroll' } as never),
    SelectRoute.update({ getParentRoute: () => root, id: '/tenant/select', path: '/tenant/select' } as never),
    home, project,
  ])
  const router = createRouter({ routeTree, history: createMemoryHistory({ initialEntries: ['/login?redirect=' + encodeURIComponent(target)] }) })
  const api = mockApi({ login: vi.fn(async () => ({ email: 'acceptance@example.test', level })) })
  const view = renderWithProviders(<RouterProvider router={router} />, { api })
  return { ...view, router, api }
}
async function submitPassword(container: HTMLElement) {
  await screen.findByRole('button', { name: '登录', exact: true })
  fireEvent.change(fieldByName(container, 'login', 'email'), { target: { value: 'acceptance@example.test' } })
  fireEvent.change(fieldByName(container, 'login', 'password'), { target: { value: TEST_PW } })
  fireEvent.click(screen.getByRole('button', { name: '登录', exact: true }))
}

describe('product authentication route chain', () => {
  for (const target of ['/projects/project-001?tab=summary#details', 'https://evil.example/landing']) {
    it('continues safely to the requested destination after tenant selection: ' + target, async () => {
      const { router, api } = renderLogin('mfa_pending', target)
      vi.mocked(api.listTenants).mockResolvedValue({ items: [{ id: 'tenant-1', slug: 'acme', name: '验收机构', status: 'active', role: 'tenant_admin' }], nextCursor: null, total: 1 })
      await router.navigate({ href: '/tenant/select?redirect=' + encodeURIComponent(target) })
      fireEvent.click(await screen.findByRole('button', { name: '进入', exact: true }))
      await screen.findByRole('heading', { name: target.startsWith('https:') ? '工作台' : '目标项目' })
      expect(router.state.location.pathname).toBe(target.startsWith('https:') ? '/' : '/projects/project-001')
    })
  }

  it('retains path, query and fragment through real route wrappers and MFA verification', async () => {
    const target = '/projects/project-001?tab=summary#details'
    const { container, router } = renderLogin('mfa_pending', target)
    await submitPassword(container)
    await screen.findByRole('button', { name: '验证并登录' })
    expect(router.state.location.search).toEqual({ redirect: target })
    fireEvent.change(fieldByName(container, 'mfa-verify', 'code'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: '验证并登录' }))
    await screen.findByRole('heading', { name: '目标项目' })
    expect(router.state.location.pathname).toBe('/projects/project-001')
    expect(router.state.location.search).toEqual({ tab: 'summary' })
    expect(router.state.location.hash).toBe('details')
  })
  it('retains the business destination when password login requires MFA enrollment', async () => {
    const target = '/projects/project-001'
    const { container, router } = renderLogin('setup', target)
    await submitPassword(container)
    await screen.findByRole('button', { name: '开始绑定' })
    expect(router.state.location.pathname).toBe('/mfa/enroll')
    expect(router.state.location.search).toEqual({ redirect: target })
  })
  for (const target of ['https://evil.example/landing', '//evil.example/landing', '/'+String.fromCharCode(92)+'evil.example']) {
    it('rejects unsafe destinations before carrying them into MFA: ' + target, async () => {
      const { container, router } = renderLogin('mfa_pending', target)
      await submitPassword(container)
      await screen.findByRole('button', { name: '验证并登录' })
      expect(router.state.location.search).toEqual({ redirect: '/' })
      fireEvent.change(fieldByName(container, 'mfa-verify', 'code'), { target: { value: '123456' } })
      fireEvent.click(screen.getByRole('button', { name: '验证并登录' }))
      await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    })
  }
})
