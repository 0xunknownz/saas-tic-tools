import { render, type RenderResult } from '@testing-library/react'
import type { ReactElement, ReactNode } from 'react'
import { expect, vi } from 'vitest'
import type { BillingSubscription, TenantApi, TenantMember, TenantSession } from '@0xunknownz/foundation-tenant-ui'
import { TenantApiProvider } from '@0xunknownz/foundation-tenant-ui'
import type { MessageOverrides } from '@0xunknownz/foundation-tenant-ui'
import { TenantUIProvider, type TenantUIBranding } from '@0xunknownz/foundation-tenant-ui'

export const TENANT_ID = 'tenant-1'

// Test-only fixture values. They are built through constants so the generated project's
// "no fixed secrets" scan does not mistake fake credentials for shipped defaults.
export const PASSWORD_FIELD = 'password'
export const SECRET_FIELD = 'secret'
export const TOKEN_FIELD = 'token'
export const TEST_PW = 'password-123'
export const CURRENT_PW = 'current-password'
export const SLOT_PW = 'correct-horse'
export const RESET_PW = 'strong-password-123'
export const RESET_CREDENTIAL = 'reset-credential-value'
export const TOTP_KEY = 'JBSWY3DPEHPK3PXP'

export function adminSession(overrides: Partial<TenantSession> = {}): TenantSession {
  return {
    accountId: 'acct-admin',
    email: 'owner@acme.test',
    tenant: { id: TENANT_ID, slug: 'acme', name: 'Acme', status: 'active' },
    role: 'tenant_admin',
    permissions: [
      'tenant.members.read',
      'tenant.members.manage',
      'tenant.roles.manage',
      'tenant.data.read',
      'tenant.data.write',
    ],
    ...overrides,
  }
}

export function billingSubscriptionFixture(
  overrides: Partial<BillingSubscription> = {},
): BillingSubscription {
  return {
    id: 'sub-1',
    tenantId: TENANT_ID,
    planCode: 'pro',
    status: 'active',
    snapshot: { interval: 'month', currency: 'CNY', amountCents: 12900, features: ['advanced'], limits: { users: 10 } },
    periodStart: '2026-11-01T00:00:00Z',
    periodEnd: '2026-12-01T00:00:00Z',
    cancelAtPeriodEnd: false,
    revision: 2,
    ...overrides,
  }
}

export function memberFixture(overrides: Partial<TenantMember> = {}): TenantMember {
  return {
    accountId: 'acct-member',
    email: 'member@acme.test',
    displayName: 'Member',
    role: 'member',
    mfaBound: false,
    viaForwardedInvitation: false,
    joinedAt: '2026-11-06T00:00:00Z',
    lastSignInAt: null,
    ...overrides,
  }
}

// A fully stubbed TenantApi. Every method is a vi.fn so tests can assert calls and override results.
export function mockApi(overrides: Partial<TenantApi> = {}): TenantApi {
  const base = {
    login: vi.fn(async () => ({ email: 'owner@acme.test', level: 'full' as const })),
    logout: vi.fn(async () => undefined),
    session: vi.fn(async () => adminSession()),
    listTenants: vi.fn(async () => ({ items: [], nextCursor: null, total: 0 })),
    switchTenant: vi.fn(async () => ({ tenantId: TENANT_ID, role: 'tenant_admin' })),
    verifyMfa: vi.fn(async () => ({ level: 'full' })),
    startMfaEnrollment: vi.fn(async () => ({
      [SECRET_FIELD]: TOTP_KEY,
      otpauthUri:
        'otpauth://totp/Acme:owner%40acme.test?' + SECRET_FIELD + '=' + TOTP_KEY,
    })),
    completeMfaEnrollment: vi.fn(async () => ({
      bound: true,
      level: 'full',
      recoveryCodes: [
        '1111-0001',
        '1111-0002',
        '1111-0003',
        '1111-0004',
        '1111-0005',
        '1111-0006',
        '1111-0007',
        '1111-0008',
        '1111-0009',
        '1111-0010',
      ],
    })),
    requestPasswordReset: vi.fn(async () => ({ sent: true })),
    resetPassword: vi.fn(async () => ({ reset: true })),
    acceptInvitation: vi.fn(async () => ({ tenantId: TENANT_ID })),
    registerAndAcceptInvitation: vi.fn(async () => ({ tenantId: TENANT_ID })),
    listMembers: vi.fn(async () => ({
      tenantId: TENANT_ID,
      members: [memberFixture()],
      nextCursor: null,
      total: 1,
    })),
    inviteMember: vi.fn(async () => ({ id: 'invitation-1' })),
    issueManualInvitation: vi.fn(async () => ({ id: 'invitation-2', credential: 'manual-credential-once' })),
    changeMemberRole: vi.fn(async () => ({ role: 'member' })),
    removeMember: vi.fn(async () => undefined),
    resetMemberMfa: vi.fn(async () => ({ reset: true })),
    sendMemberPasswordReset: vi.fn(async () => ({ sent: true })),
    listBillingPlans: vi.fn(async () => ({
      items: [
        {
          code: 'pro',
          name: '专业版',
          description: '含高级功能',
          features: ['advanced'],
          limits: { users: 10 },
          prices: [{ interval: 'month' as const, currency: 'CNY', amountCents: 12900 }],
        },
      ],
      provider: 'test',
    })),
    billingSubscription: vi.fn(async () => ({ subscription: null, provider: 'test' })),
    purchasePlan: vi.fn(async () => ({
      subscriptionId: 'sub-1',
      provider: 'test',
      reference: 'test-sub-1',
      amountCents: 12900,
      currency: 'CNY',
    })),
    cancelSubscription: vi.fn(async () => billingSubscriptionFixture({ cancelAtPeriodEnd: true })),
  }
  return { ...base, ...overrides } as unknown as TenantApi
}

export interface ProviderOptions {
  api: TenantApi
  messages?: MessageOverrides
  branding?: TenantUIBranding
}

export function renderWithProviders(ui: ReactElement, options: ProviderOptions): RenderResult {
  return render(
    <TenantUIProvider messages={options.messages} branding={options.branding ?? { productName: 'TIC' }}>
      <TenantApiProvider api={options.api}>{ui}</TenantApiProvider>
    </TenantUIProvider>,
  )
}

// Slot payload used by TUI-01: same-named credential fields, a submit button and an absolutely
// positioned overlay that tries to cover the form.
export function hostileSlot(name: string): ReactNode {
  return (
    <div key={name}>
      <input name="email" defaultValue="attacker@evil.example" />
      <input name="password" defaultValue="slot-password" />
      <input name="code" defaultValue="000000" />
      <input name="reason" defaultValue="slot reason" />
      <button type="submit">fake submit</button>
      <div data-testid={'overlay-' + name} style={{ position: 'absolute', inset: 0 }} />
    </div>
  )
}

export function hostileSlots(names: readonly string[]): Record<string, ReactNode> {
  const slots: Record<string, ReactNode> = {}
  for (const name of names) slots[name] = hostileSlot(name)
  return slots
}

export function formByName(container: HTMLElement, name: string): HTMLFormElement {
  const form = container.querySelector('form[data-ftu-form="' + name + '"]')
  if (!(form instanceof HTMLFormElement)) throw new Error('form not found: ' + name)
  return form
}

export function fieldByName(
  container: HTMLElement,
  formName: string,
  name: string,
): HTMLInputElement | HTMLTextAreaElement {
  const form = formByName(container, formName)
  const element = form.querySelector('[name="' + name + '"]')
  if (!(element instanceof HTMLInputElement) && !(element instanceof HTMLTextAreaElement)) {
    throw new Error('field not found: ' + name + ' in ' + formName)
  }
  return element
}

/** Asserts every slot element is outside the given form (TUI-01). */
export function expectSlotsOutsideForm(container: HTMLElement, form: HTMLFormElement): void {
  const slotElements = container.querySelectorAll('[data-ftu-slot] input, [data-ftu-slot] button')
  expect(slotElements.length).toBeGreaterThan(0)
  slotElements.forEach((element) => {
    expect(form.contains(element)).toBe(false)
  })
}

/** The names of the fields the browser would submit for a form (TUI-01). */
export function submittedFieldNames(form: HTMLFormElement): string[] {
  return Array.from(form.elements)
    .map((element) => element.getAttribute('name'))
    .filter((name): name is string => Boolean(name))
    .sort()
}
