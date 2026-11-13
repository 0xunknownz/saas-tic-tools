import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import {
  TenantApiError,
  type InvitationResult,
  type LoginResult,
  type MemberCredentialResult,
  type MfaEnrollment,
  type MfaEnrollmentComplete,
  type TenantApi,
  type TenantMember,
  type TenantSession,
} from '@0xunknownz/foundation-tenant-ui'
import { AcceptInvitationPage } from '@0xunknownz/foundation-tenant-ui'
import { ForgotPasswordPage } from '@0xunknownz/foundation-tenant-ui'
import { LoginPage } from '@0xunknownz/foundation-tenant-ui'
import { MembersPage } from '@0xunknownz/foundation-tenant-ui'
import { MfaEnrollPage } from '@0xunknownz/foundation-tenant-ui'
import { MfaVerifyPage } from '@0xunknownz/foundation-tenant-ui'
import { ResetPasswordPage } from '@0xunknownz/foundation-tenant-ui'
import { PASSWORD_FIELD, SECRET_FIELD, TOKEN_FIELD, TOTP_KEY, fieldByName, formByName, renderWithProviders } from './helpers'

interface FakeMember {
  accountId: string
  email: string
  role: string
  password: string
  mfaBound: boolean
  sessionsRevoked: boolean
  recoveryCodes: string[]
}

interface FakeInvitation {
  token: string
  email: string
  role: string
  accepted: boolean
}

const ADMIN_PW = 'admin-password-1'

const RECOVERY = [
  'R-0001', 'R-0002', 'R-0003', 'R-0004', 'R-0005',
  'R-0006', 'R-0007', 'R-0008', 'R-0009', 'R-0010',
]

function unauthenticated(): TenantApiError {
  return new TenantApiError(401, 'UNAUTHENTICATED', 'sign in first')
}

// In-memory API that mirrors the tenant-realm contract closely enough to drive AC-022 end to end.
function createFakeApi() {
  const members = new Map<string, FakeMember>()
  const invitations = new Map<string, FakeInvitation>()
  const resetTokens = new Map<string, { email: string; used: boolean }>()
  let currentAccountId: string | null = 'acct-admin'
  let sequence = 0

  members.set('acct-admin', {
    accountId: 'acct-admin',
    email: 'owner@acme.test',
    role: 'tenant_admin',
    [PASSWORD_FIELD]: ADMIN_PW,
    mfaBound: true,
    sessionsRevoked: false,
    recoveryCodes: [],
  })

  function current(): FakeMember {
    if (!currentAccountId) throw unauthenticated()
    const member = members.get(currentAccountId)
    if (!member) throw unauthenticated()
    return member
  }

  function requireAdmin(): FakeMember {
    const member = current()
    if (member.role !== 'tenant_admin') {
      throw new TenantApiError(403, 'PERMISSION_DENIED', 'tenant administrators only')
    }
    return member
  }

  function memberView(member: FakeMember): TenantMember {
    return {
      accountId: member.accountId,
      email: member.email,
      displayName: member.email.split('@')[0],
      role: member.role,
      mfaBound: member.mfaBound,
      viaForwardedInvitation: false,
      joinedAt: '2026-11-06T00:00:00Z',
      lastSignInAt: null,
    }
  }

  const api: TenantApi = {
    async login(input): Promise<LoginResult> {
      const member = Array.from(members.values()).find((candidate) => candidate.email === input.email)
      if (!member || member.password !== input.password) {
        throw new TenantApiError(401, 'INVALID_CREDENTIALS', 'invalid credentials')
      }
      currentAccountId = member.accountId
      return { email: member.email, level: member.mfaBound ? 'mfa_pending' : 'full' }
    },
    async logout(): Promise<void> {
      currentAccountId = null
    },
    async session(): Promise<TenantSession> {
      const member = current()
      return {
        accountId: member.accountId,
        email: member.email,
        tenant: { id: 'tenant-1', slug: 'acme', name: 'Acme', status: 'active' },
        role: member.role,
        permissions:
          member.role === 'tenant_admin'
            ? ['tenant.members.read', 'tenant.members.manage', 'tenant.roles.manage']
            : ['tenant.data.read'],
      }
    },
    async listTenants() {
      return { items: [{ id: 'tenant-1', slug: 'acme', name: 'Acme', status: 'active', role: current().role }], nextCursor: null, total: 1 }
    },
    async switchTenant(tenantId: string) {
      return { tenantId, role: current().role }
    },
    async verifyMfa(code: string) {
      const member = current()
      const valid = code === '123456' || member.recoveryCodes.includes(code)
      if (!valid) throw new TenantApiError(401, 'MFA_INVALID', 'invalid code')
      return { level: 'full' }
    },
    async startMfaEnrollment(password: string): Promise<MfaEnrollment> {
      const member = current()
      if (member.password !== password) throw new TenantApiError(401, 'INVALID_CREDENTIALS', 'invalid password')
      if (member.mfaBound) throw new TenantApiError(409, 'MFA_ALREADY_BOUND', 'already bound')
      return { [SECRET_FIELD]: TOTP_KEY, otpauthUri: 'otpauth://totp/Acme:' + member.email }
    },
    async completeMfaEnrollment(code: string): Promise<MfaEnrollmentComplete> {
      const member = current()
      if (code !== '123456') throw new TenantApiError(401, 'MFA_INVALID', 'invalid code')
      member.mfaBound = true
      member.recoveryCodes = [...RECOVERY]
      return { bound: true, level: 'full', recoveryCodes: [...RECOVERY] }
    },
    async requestPasswordReset(email: string) {
      sequence += 1
      resetTokens.set('reset-token-' + sequence, { email, used: false })
      return { sent: true }
    },
    async resetPassword(input) {
      const credential = resetTokens.get(input.token)
      if (!credential || credential.used) throw new TenantApiError(400, 'RESET_TOKEN_INVALID', 'invalid token')
      const member = Array.from(members.values()).find((candidate) => candidate.email === credential.email)
      if (!member) throw new TenantApiError(400, 'RESET_TOKEN_INVALID', 'invalid token')
      if (member.mfaBound && input.code !== '123456') {
        throw new TenantApiError(401, 'MFA_INVALID', 'MFA code required')
      }
      member.password = input.password
      member.sessionsRevoked = true
      credential.used = true
      return { reset: true }
    },
    async acceptInvitation(input): Promise<InvitationResult> {
      const invitation = invitations.get(input.token)
      if (!invitation || invitation.accepted) throw new TenantApiError(400, 'INVITATION_INVALID', 'invalid invitation')
      const member = current()
      if (member.email !== invitation.email) {
        throw new TenantApiError(403, 'INVITATION_EMAIL_MISMATCH', 'email mismatch')
      }
      invitation.accepted = true
      return { tenantId: 'tenant-1' }
    },
    async registerAndAcceptInvitation(input): Promise<InvitationResult> {
      const invitation = invitations.get(input.token)
      if (!invitation || invitation.accepted) throw new TenantApiError(400, 'INVITATION_INVALID', 'invalid invitation')
      if (invitation.email !== input.email) {
        throw new TenantApiError(403, 'INVITATION_EMAIL_MISMATCH', 'email mismatch')
      }
      if (Array.from(members.values()).some((candidate) => candidate.email === input.email)) {
        throw new TenantApiError(409, 'ACCOUNT_EXISTS', 'account exists')
      }
      const accountId = 'acct-' + input.email
      members.set(accountId, {
        accountId,
        email: input.email,
        role: invitation.role,
        password: input.password,
        mfaBound: false,
        sessionsRevoked: false,
        recoveryCodes: [],
      })
      invitation.accepted = true
      currentAccountId = accountId
      return { tenantId: 'tenant-1' }
    },
    async listMembers() {
      requireAdmin()
      return {
        tenantId: 'tenant-1',
        members: Array.from(members.values()).map(memberView),
        nextCursor: null,
        total: members.size,
      }
    },
    async inviteMember(email: string) {
      requireAdmin()
      sequence += 1
      const credential = 'invite-credential-' + sequence
      invitations.set(credential, { [TOKEN_FIELD]: credential, email, role: 'member', accepted: false })
      return { id: credential }
    },
    async issueManualInvitation(email: string) {
      requireAdmin()
      sequence += 1
      const credential = 'manual-invite-' + sequence
      invitations.set(credential, { [TOKEN_FIELD]: credential, email, role: 'member', accepted: false })
      return { id: credential, credential }
    },
    async changeMemberRole(accountId: string, role: string) {
      requireAdmin()
      const member = members.get(accountId)
      if (!member) throw new TenantApiError(404, 'ACCOUNT_NOT_FOUND', 'unknown account')
      member.role = role
      return { role }
    },
    async removeMember(accountId: string) {
      requireAdmin()
      members.delete(accountId)
    },
    async resetMemberMfa(accountId: string, reason: string): Promise<MemberCredentialResult> {
      const admin = requireAdmin()
      if (admin.accountId === accountId) throw new TenantApiError(403, 'SELF_RESET_NOT_ALLOWED', 'self reset')
      const member = members.get(accountId)
      if (!member) throw new TenantApiError(404, 'ACCOUNT_NOT_FOUND', 'unknown account')
      if (reason.trim() === '') throw new TenantApiError(400, 'TENANT_REASON_REQUIRED', 'reason required')
      member.mfaBound = false
      member.recoveryCodes = []
      member.sessionsRevoked = true
      return { reset: true }
    },
    async sendMemberPasswordReset(accountId: string, reason: string): Promise<MemberCredentialResult> {
      const admin = requireAdmin()
      if (admin.accountId === accountId) throw new TenantApiError(403, 'SELF_RESET_NOT_ALLOWED', 'self reset')
      if (reason.trim() === '') throw new TenantApiError(400, 'TENANT_REASON_REQUIRED', 'reason required')
      sequence += 1
      const member = members.get(accountId)
      if (!member) throw new TenantApiError(404, 'ACCOUNT_NOT_FOUND', 'unknown account')
      resetTokens.set('admin-reset-' + sequence, { email: member.email, used: false })
      return { sent: true }
    },
  }

  return {
    api,
    members,
    invitations,
    resetTokens,
    setCurrent(accountId: string | null) {
      currentAccountId = accountId
    },
  }
}

async function renderMembers(fake: ReturnType<typeof createFakeApi>) {
  const view = renderWithProviders(<MembersPage navigate={vi.fn()} />, { api: fake.api })
  await waitFor(() => expect(view.container.querySelector('[data-ftu-member-table]')).toBeTruthy())
  return view
}

describe('AC-022 account and membership chain (mock API)', () => {
  it('invites, accepts, resets the password, binds MFA, signs in with MFA and resets credentials as an administrator', async () => {
    const fake = createFakeApi()

    // 1. Tenant administrator invites a member.
    let view = await renderMembers(fake)
    fireEvent.change(fieldByName(view.container, 'members-invite', 'email'), {
      target: { value: 'newbie@acme.test' },
    })
    fireEvent.click(formByName(view.container, 'members-invite').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(fake.invitations.size).toBe(1))
    const invitation = Array.from(fake.invitations.values())[0]
    expect(invitation?.email).toBe('newbie@acme.test')
    view.unmount()

    // 2. The invitee registers with the invitation email and joins the tenant.
    fake.setCurrent(null)
    view = renderWithProviders(<AcceptInvitationPage token={invitation!.token} navigate={vi.fn()} />, { api: fake.api })
    await waitFor(() => expect(view.container.querySelector('form[data-ftu-form="invitation-register"]')).toBeTruthy())
    fireEvent.change(fieldByName(view.container, 'invitation-register', 'email'), {
      target: { value: 'newbie@acme.test' },
    })
    fireEvent.change(fieldByName(view.container, 'invitation-register', 'password'), {
      target: { value: 'initial-password-1' },
    })
    fireEvent.click(formByName(view.container, 'invitation-register').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(fake.members.size).toBe(2))
    expect(fake.members.get('acct-newbie@acme.test')?.role).toBe('member')
    expect(screen.getByText('已加入租户。')).toBeTruthy()
    view.unmount()

    // 3. The member requests a password reset (the response is identical for every address).
    view = renderWithProviders(<ForgotPasswordPage navigate={vi.fn()} />, { api: fake.api })
    fireEvent.change(fieldByName(view.container, 'password-request', 'email'), {
      target: { value: 'newbie@acme.test' },
    })
    fireEvent.click(formByName(view.container, 'password-request').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(fake.resetTokens.size).toBe(1))
    const resetToken = Array.from(fake.resetTokens.keys())[0]!
    view.unmount()

    // 4. The reset link sets a new password.
    view = renderWithProviders(<ResetPasswordPage token={resetToken} navigate={vi.fn()} />, { api: fake.api })
    fireEvent.change(fieldByName(view.container, 'password-reset', 'password'), {
      target: { value: 'new-password-123' },
    })
    fireEvent.change(fieldByName(view.container, 'password-reset', 'confirm'), {
      target: { value: 'new-password-123' },
    })
    fireEvent.click(formByName(view.container, 'password-reset').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(fake.members.get('acct-newbie@acme.test')?.password).toBe('new-password-123'))
    expect(fake.members.get('acct-newbie@acme.test')?.sessionsRevoked).toBe(true)
    view.unmount()

    // 5. First sign-in with the new password (no authenticator bound yet).
    const firstNavigate = vi.fn()
    view = renderWithProviders(<LoginPage navigate={firstNavigate} />, { api: fake.api })
    fireEvent.change(fieldByName(view.container, 'login', 'email'), { target: { value: 'newbie@acme.test' } })
    fireEvent.change(fieldByName(view.container, 'login', 'password'), {
      target: { value: 'new-password-123' },
    })
    fireEvent.click(formByName(view.container, 'login').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(firstNavigate).toHaveBeenCalledWith('/'))
    view.unmount()

    // 6. The member binds an authenticator and saves the one-time recovery codes.
    const enrollNavigate = vi.fn()
    view = renderWithProviders(<MfaEnrollPage navigate={enrollNavigate} />, { api: fake.api })
    fireEvent.change(fieldByName(view.container, 'mfa-enroll-password', 'password'), {
      target: { value: 'new-password-123' },
    })
    fireEvent.click(formByName(view.container, 'mfa-enroll-password').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(view.container.querySelector('[data-ftu-mfa-secret]')).toBeTruthy())
    fireEvent.change(fieldByName(view.container, 'mfa-enroll-code', 'code'), { target: { value: '123456' } })
    fireEvent.click(formByName(view.container, 'mfa-enroll-code').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(view.container.querySelector('[data-ftu-recovery-codes]')).toBeTruthy())
    fireEvent.click(view.container.querySelector('[data-ftu-recovery-saved]') as HTMLElement)
    fireEvent.click(view.container.querySelector('[data-ftu-security="recovery-continue"]') as HTMLElement)
    await waitFor(() => expect(enrollNavigate).toHaveBeenCalledWith('/'))
    expect(fake.members.get('acct-newbie@acme.test')?.mfaBound).toBe(true)
    view.unmount()

    // 7. The next sign-in requires the second factor.
    const mfaLoginNavigate = vi.fn()
    view = renderWithProviders(<LoginPage navigate={mfaLoginNavigate} />, { api: fake.api })
    fireEvent.change(fieldByName(view.container, 'login', 'email'), { target: { value: 'newbie@acme.test' } })
    fireEvent.change(fieldByName(view.container, 'login', 'password'), {
      target: { value: 'new-password-123' },
    })
    fireEvent.click(formByName(view.container, 'login').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(mfaLoginNavigate).toHaveBeenCalledWith('/mfa/verify'))
    view.unmount()

    // 8. The MFA code completes sign-in.
    const verifyNavigate = vi.fn()
    view = renderWithProviders(<MfaVerifyPage navigate={verifyNavigate} />, { api: fake.api })
    fireEvent.change(fieldByName(view.container, 'mfa-verify', 'code'), { target: { value: '123456' } })
    fireEvent.click(formByName(view.container, 'mfa-verify').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(verifyNavigate).toHaveBeenCalledWith('/'))
    view.unmount()

    // 9. The tenant administrator resets the member's credentials (MFA and password link).
    fake.setCurrent('acct-admin')
    view = await renderMembers(fake)
    const row = view.container.querySelector('[data-ftu-member="acct-newbie@acme.test"]') as HTMLElement
    expect(row).toBeTruthy()
    fireEvent.click(within(row).getByText('重置 MFA'))
    fireEvent.change(fieldByName(view.container, 'members-credential-reset', 'reason'), {
      target: { value: 'lost device, identity verified by phone' },
    })
    fireEvent.click(formByName(view.container, 'members-credential-reset').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(fake.members.get('acct-newbie@acme.test')?.mfaBound).toBe(false))
    expect(fake.members.get('acct-newbie@acme.test')?.sessionsRevoked).toBe(true)
    expect(fake.members.get('acct-newbie@acme.test')?.recoveryCodes.length).toBe(0)

    const refreshedRow = view.container.querySelector('[data-ftu-member="acct-newbie@acme.test"]') as HTMLElement
    fireEvent.click(within(refreshedRow).getByText('发送密码重置'))
    fireEvent.change(fieldByName(view.container, 'members-credential-reset', 'reason'), {
      target: { value: 'password lost, identity verified by phone' },
    })
    fireEvent.click(formByName(view.container, 'members-credential-reset').querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(fake.resetTokens.size).toBe(2))
    view.unmount()
  })
})
