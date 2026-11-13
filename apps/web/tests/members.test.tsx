import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { MembersPage } from '@0xunknownz/foundation-tenant-ui'
import { adminSession, fieldByName, memberFixture, mockApi, renderWithProviders } from './helpers'

describe('members page', () => {
  it('issues a one-time manual invitation credential', async () => {
    const api = mockApi()
    const { container } = renderWithProviders(<MembersPage navigate={vi.fn()} />, { api })
    await waitFor(() => expect(container.querySelector('[data-ftu-member-table]')).toBeTruthy())

    fireEvent.change(fieldByName(container, 'members-invite', 'email'), {
      target: { value: 'newbie@acme.test' },
    })
    fireEvent.click(container.querySelector('[data-ftu-action="manual-invitation"]') as HTMLElement)
    await waitFor(() => expect(vi.mocked(api.issueManualInvitation)).toHaveBeenCalledWith('newbie@acme.test'))
    await waitFor(() => expect(container.querySelector('[data-ftu-invite-credential]')?.textContent).toBe('manual-credential-once'))
  })

  it('demotes a member, disables promotion and keeps self actions disabled', async () => {
    const api = mockApi({
      session: vi.fn(async () => adminSession()),
      listMembers: vi.fn(async () => ({
        tenantId: 'tenant-1',
        members: [
          memberFixture(),
          memberFixture({ accountId: 'acct-lead', email: 'lead@acme.test', role: 'tenant_admin' }),
          memberFixture({ accountId: 'acct-admin', email: 'owner@acme.test', role: 'tenant_admin' }),
        ],
        nextCursor: null,
        total: 3,
      })),
    })
    const { container } = renderWithProviders(<MembersPage navigate={vi.fn()} />, { api })
    await waitFor(() => expect(container.querySelector('[data-ftu-member="acct-lead"]')).toBeTruthy())

    const select = container.querySelector('[data-ftu-role-select="acct-lead"]') as HTMLSelectElement
    const promotion = select.querySelector('option[value="tenant_admin"]') as HTMLOptionElement
    expect(promotion.disabled).toBe(true)
    fireEvent.change(select, { target: { value: 'member' } })
    await waitFor(() => expect(vi.mocked(api.changeMemberRole)).toHaveBeenCalledWith('acct-lead', 'member'))

    const selfRow = container.querySelector('[data-ftu-member="acct-admin"]') as HTMLElement
    expect(within(selfRow).queryByText('重置 MFA')).toBeNull()
    expect(container.querySelector('[data-ftu-role-select="acct-admin"]')).toBeNull()
  })

  it('removes a member after confirmation', async () => {
    const api = mockApi()
    const { container } = renderWithProviders(<MembersPage navigate={vi.fn()} />, { api })
    await waitFor(() => expect(container.querySelector('[data-ftu-member="acct-member"]')).toBeTruthy())

    fireEvent.click(container.querySelector('[data-ftu-action="remove-member"]') as HTMLElement)
    const form = container.querySelector('form[data-ftu-form="members-remove"]') as HTMLFormElement
    expect(form).toBeTruthy()
    fireEvent.click(form.querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(vi.mocked(api.removeMember)).toHaveBeenCalledWith('acct-member'))
  })

  it('shows the forbidden state instead of the table for a member without permission', async () => {
    const api = mockApi({
      session: vi.fn(async () => adminSession({ role: 'member', permissions: ['tenant.data.read'] })),
    })
    const { container } = renderWithProviders(<MembersPage navigate={vi.fn()} />, { api })
    await waitFor(() => expect(container.textContent).toContain('只有租户管理员可以管理成员'))
    expect(container.querySelector('[data-ftu-member-table]')).toBeNull()
    expect(screen.queryAllByText('邀请成员').length).toBe(0)
  })
})
