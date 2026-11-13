import { fireEvent, screen, waitFor } from '@testing-library/react'
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router'
import { TenantApiError } from '@0xunknownz/foundation-tenant-ui'
import { describe, expect, it, vi } from 'vitest'
import { WorkbenchBoundary } from '../src/features/workbench/Shell'
import { WorkbenchPage } from '../src/features/workbench/WorkbenchPage'
import { allNavItems, activeGroup, navGroups, toolItems } from '../src/features/workbench/navigation'
import { mockApi, renderWithProviders } from './helpers'

function renderWorkbench(path = '/', api = mockApi()) {
  const root = createRootRoute({ component: () => <WorkbenchBoundary><Outlet /></WorkbenchBoundary> })
  const route = createRoute({ getParentRoute: () => root, path: '$', component: WorkbenchPage })
  const home = createRoute({ getParentRoute: () => root, path: '/', component: WorkbenchPage })
  const router = createRouter({ routeTree: root.addChildren([route, home]), history: createMemoryHistory({ initialEntries: [path] }) })
  renderWithProviders(<RouterProvider router={router} />, { api })
  return router
}
describe('TIC workbench integration', () => {
  it('guards demo content behind the existing session and preserves the return URL', async () => {
    renderWorkbench('/projects', mockApi({ session: vi.fn(async () => { throw new TenantApiError(401, 'UNAUTHENTICATED', '') }) }))
    const link = await screen.findByRole('link', { name: '前往登录' })
    expect(link.getAttribute('href')).toBe('/login?redirect=%2Fprojects')
    expect(screen.queryByRole('navigation', { name: '主导航' })).toBeNull()
    expect(screen.queryByText('项目快捷入口')).toBeNull()
  })
  it('allows retrying a failed session without exposing content', async () => {
    const api = mockApi()
    vi.mocked(api.session).mockRejectedValueOnce(new TenantApiError(0, 'NETWORK_ERROR', 'offline'))
    renderWorkbench('/', api)
    fireEvent.click(await screen.findByRole('button', { name: '重试' }))
    expect(await screen.findByRole('heading', { name: 'TIC工具箱' })).toBeTruthy()
  })
  it('uses the same URLs for tool shortcuts and navigation and keeps admin boundaries', async () => {
    const router = renderWorkbench()
    await screen.findByRole('heading', { name: 'TIC工具箱' })
    for (const tool of toolItems) expect(screen.getByRole('link', { name: new RegExp(tool.label) }).getAttribute('href')).toBe(tool.href)
    expect(screen.getByRole('link', { name: '成员管理' }).getAttribute('href')).toBe('/console/members')
    expect(screen.getByRole('link', { name: '账单管理' }).getAttribute('href')).toBe('/console/billing')
    expect(screen.getByRole('link', { name: '平台管理端' }).getAttribute('href')).toBe('/admin/')
    fireEvent.click(screen.getByRole('link', { name: /国家标准/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/standards'))
    expect(screen.getByRole('link', { name: '数据工具中心' }).getAttribute('aria-current')).toBe('page')
    expect(screen.getByRole('link', { name: '国家标准' }).getAttribute('aria-current')).toBe('page')
    await router.navigate({ href: '/no-such-page' })
    expect(await screen.findByRole('heading', { name: '页面未找到' })).toBeTruthy()
  })
  it('does not pretend a failed logout succeeded', async () => {
    renderWorkbench('/', mockApi({ logout: vi.fn(async () => { throw Error('offline') }) }))
    fireEvent.click(await screen.findByRole('button', { name: '退出' }))
    expect((await screen.findByRole('alert')).textContent).toContain('退出失败')
    expect(screen.getByRole('heading', { name: 'TIC工具箱' })).toBeTruthy()
  })
  it('has a complete unambiguous catalog including all original operations items', () => {
    const unique = new Set(allNavItems.map(item => item.href))
    expect(unique.size).toBe(allNavItems.length - 1) // project group and project-library share their destination
    expect(navGroups.find(g => g.href === '/operations')?.items).toHaveLength(10)
    for (const entry of allNavItems) expect(activeGroup(entry.href)).toBeTruthy()
    expect(activeGroup('/standards/demo-1')?.label).toBe('数据工具中心')
  })
})
