import { fireEvent, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { TenantApiError } from '@0xunknownz/foundation-tenant-ui'
import { TENANT_SWITCH_SLOTS, TenantSwitchPage } from '@0xunknownz/foundation-tenant-ui'
import { adminSession, fieldByName, formByName, hostileSlots, mockApi, renderWithProviders } from './helpers'

const memberships = [
  { id: 'tenant-1', slug: 'acme', name: 'Acme', status: 'active', role: 'tenant_admin' },
  { id: 'tenant-2', slug: 'beta', name: 'Beta', status: 'frozen', role: 'member' },
]

describe('tenant switching', () => {
  it('lists memberships and switches through the session endpoint', async () => {
    const api = mockApi({ listTenants: vi.fn(async () => ({ items: memberships, nextCursor: null, total: 2 })) })
    const navigate = vi.fn()
    renderWithProviders(<TenantSwitchPage navigate={navigate} />, { api })

    await waitFor(() => expect(screen.getByText('Beta')).toBeTruthy())
    expect(screen.getByText('冻结')).toBeTruthy()
    const button = document.querySelector('[data-ftu-tenant-id="tenant-2"]')
    expect(button).toBeTruthy()
    fireEvent.click(button as HTMLElement)
    await waitFor(() => expect(vi.mocked(api.switchTenant)).toHaveBeenCalledWith('tenant-2'))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/'))
  })

  it('offers a manual fallback when the membership list endpoint is unavailable', async () => {
    const api = mockApi({
      listTenants: vi.fn(async () => {
        throw new TenantApiError(404, 'HTTP_404', 'not found')
      }),
    })
    const { container } = renderWithProviders(<TenantSwitchPage navigate={vi.fn()} />, { api })

    await waitFor(() => expect(container.querySelector('form[data-ftu-form="tenant-switch-manual"]')).toBeTruthy())
    fireEvent.change(fieldByName(container, 'tenant-switch-manual', 'tenantId'), {
      target: { value: 'tenant-9' },
    })
    fireEvent.click(formByName(container, 'tenant-switch-manual').querySelector('button[type="submit"]') as HTMLElement)
    await waitFor(() => expect(vi.mocked(api.switchTenant)).toHaveBeenCalledWith('tenant-9'))
  })

  it('shows the empty state slot when the account has no tenant', async () => {
    const api = mockApi({
      listTenants: vi.fn(async () => ({ items: [], nextCursor: null, total: 0 })),
      session: vi.fn(async () => adminSession({ tenant: null, role: null, permissions: [] })),
    })
    const { container } = renderWithProviders(
      <TenantSwitchPage slots={hostileSlots(TENANT_SWITCH_SLOTS)} navigate={vi.fn()} />,
      { api },
    )
    await waitFor(() => expect(container.querySelector('[data-ftu-slot="emptyState"]')).toBeTruthy())
    expect(container.textContent).toContain('尚未加入任何租户')
    expect(container.querySelector('[data-ftu-slot="emptyState"]')?.closest('form')).toBeNull()
  })
})
