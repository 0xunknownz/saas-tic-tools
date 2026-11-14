import { act, cleanup, fireEvent, screen, waitFor } from '@testing-library/react'
import { createMemoryHistory, createRootRoute, createRoute, createRouter, RouterProvider } from '@tanstack/react-router'
import { afterEach, expect, it } from 'vitest'
import { mockApi, renderWithProviders } from '../../web/tests/helpers'
import { TenantLayout } from '../src/navigation'

afterEach(cleanup)
it('keeps the console base, query and hash in tenant switching across in-app navigation and history', async () => {
  const root = createRootRoute({ component: () => <TenantLayout>管理内容</TenantLayout> })
  const members = createRoute({ getParentRoute: () => root, path: '/members' })
  const billing = createRoute({ getParentRoute: () => root, path: '/billing' })
  const history = createMemoryHistory({ initialEntries: ['/console/members?tab=access#team'] })
  const router = createRouter({ routeTree: root.addChildren([members, billing]), basepath: '/console', history })
  renderWithProviders(<RouterProvider router={router}/>, { api: mockApi() })
  const switchLink = await screen.findByRole('link', { name: '切换租户' })
  expect(switchLink.getAttribute('href')).toBe('/tenant/select?redirect=' + encodeURIComponent('/console/members?tab=access#team'))
  fireEvent.click(screen.getByRole('link', { name: '账单管理' }))
  await waitFor(() => expect(switchLink.getAttribute('href')).toBe('/tenant/select?redirect=' + encodeURIComponent('/console/billing')))
  await act(async () => { history.back() })
  await waitFor(() => expect(switchLink.getAttribute('href')).toBe('/tenant/select?redirect=' + encodeURIComponent('/console/members?tab=access#team')))
  expect(screen.getByRole('link', { name: '返回工作台' }).getAttribute('href')).toBe('/')
})
