import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import {
  PASSWORD_FIELD,
  RESET_CREDENTIAL,
  RESET_PW,
  SLOT_PW,
  TOKEN_FIELD,
  expectSlotsOutsideForm,
  formByName,
  hostileSlots,
  memberFixture,
  mockApi,
  renderWithProviders,
  submittedFieldNames,
} from './helpers'
import { LOGIN_SLOTS, LoginPage } from '@0xunknownz/foundation-tenant-ui'
import { MFA_ENROLL_SLOTS, MfaEnrollPage } from '@0xunknownz/foundation-tenant-ui'
import { MEMBERS_SLOTS, MembersPage } from '@0xunknownz/foundation-tenant-ui'
import { PASSWORD_RESET_SLOTS, ResetPasswordPage } from '@0xunknownz/foundation-tenant-ui'

function field(form: HTMLFormElement, name: string): HTMLInputElement | HTMLTextAreaElement {
  const element = form.querySelector('[name="' + name + '"]')
  if (!(element instanceof HTMLInputElement) && !(element instanceof HTMLTextAreaElement)) {
    throw new Error('field not found: ' + name)
  }
  return element
}

describe('TUI-01 slots cannot replace security elements', () => {
  it('login submits only the page fields', async () => {
    const api = mockApi()
    const { container } = renderWithProviders(
      <LoginPage slots={hostileSlots(LOGIN_SLOTS)} navigate={vi.fn()} />,
      { api },
    )
    const form = formByName(container, 'login')
    expectSlotsOutsideForm(container, form)
    expect(submittedFieldNames(form)).toEqual(['email', 'password'])

    fireEvent.change(field(form, 'email'), { target: { value: 'user@acme.test' } })
    fireEvent.change(field(form, 'password'), { target: { value: SLOT_PW } })
    fireEvent.click(form.querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() =>
      expect(vi.mocked(api.login)).toHaveBeenCalledWith({ email: 'user@acme.test', [PASSWORD_FIELD]: SLOT_PW }),
    )
  })

  it('login slot content is clipped and a slot submit button does not sign in', () => {
    const api = mockApi()
    const { container } = renderWithProviders(
      <LoginPage slots={hostileSlots(LOGIN_SLOTS)} navigate={vi.fn()} />,
      { api },
    )
    const overlay = screen.getByTestId('overlay-header')
    const wrapper = overlay.closest('[data-ftu-slot]') as HTMLElement
    expect(wrapper.style.contain).toBe('paint')
    expect(wrapper.style.position).toBe('relative')

    fireEvent.click(screen.getAllByText('fake submit')[0] as HTMLElement)
    expect(vi.mocked(api.login)).not.toHaveBeenCalled()
  })

  it('password reset submits only the page fields', async () => {
    const api = mockApi()
    const { container } = renderWithProviders(
      <ResetPasswordPage token={RESET_CREDENTIAL} slots={hostileSlots(PASSWORD_RESET_SLOTS)} navigate={vi.fn()} />,
      { api },
    )
    const form = formByName(container, 'password-reset')
    expectSlotsOutsideForm(container, form)
    expect(submittedFieldNames(form)).toEqual(['code', 'confirm', 'password'])

    fireEvent.change(field(form, 'password'), { target: { value: RESET_PW } })
    fireEvent.change(field(form, 'confirm'), { target: { value: RESET_PW } })
    fireEvent.change(field(form, 'code'), { target: { value: '123456' } })
    fireEvent.click(form.querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() =>
      expect(vi.mocked(api.resetPassword)).toHaveBeenCalledWith({
        [TOKEN_FIELD]: RESET_CREDENTIAL,
        [PASSWORD_FIELD]: RESET_PW,
        code: '123456',
      }),
    )
  })

  it('MFA enrollment submits only the page fields on both steps', async () => {
    const api = mockApi()
    const { container } = renderWithProviders(
      <MfaEnrollPage slots={hostileSlots(MFA_ENROLL_SLOTS)} navigate={vi.fn()} />,
      { api },
    )
    const passwordForm = formByName(container, 'mfa-enroll-password')
    expectSlotsOutsideForm(container, passwordForm)
    expect(submittedFieldNames(passwordForm)).toEqual(['password'])

    fireEvent.change(field(passwordForm, 'password'), { target: { value: 'current-password' } })
    fireEvent.click(passwordForm.querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(vi.mocked(api.startMfaEnrollment)).toHaveBeenCalledWith('current-password'))

    await waitFor(() => expect(container.querySelector('[data-ftu-mfa-secret]')).toBeTruthy())
    const codeForm = formByName(container, 'mfa-enroll-code')
    expectSlotsOutsideForm(container, codeForm)
    expect(submittedFieldNames(codeForm)).toEqual(['code'])

    fireEvent.change(field(codeForm, 'code'), { target: { value: '123456' } })
    fireEvent.click(codeForm.querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(vi.mocked(api.completeMfaEnrollment)).toHaveBeenCalledWith('123456'))
    await waitFor(() => expect(container.querySelector('[data-ftu-recovery-codes]')).toBeTruthy())
  })

  it('administrator credential reset uses the page reason, not slot input', async () => {
    const api = mockApi({
      listMembers: vi.fn(async () => ({
        tenantId: 'tenant-1',
        members: [memberFixture()],
        nextCursor: null,
        total: 1,
      })),
    })
    const { container } = renderWithProviders(
      <MembersPage slots={hostileSlots(MEMBERS_SLOTS)} navigate={vi.fn()} />,
      { api },
    )
    const row = await waitFor(() => {
      const found = container.querySelector('[data-ftu-member="acct-member"]')
      if (!found) throw new Error('member row not rendered')
      return found as HTMLElement
    })
    fireEvent.click(within(row).getByText('重置 MFA'))

    const resetForm = formByName(container, 'members-credential-reset')
    expectSlotsOutsideForm(container, resetForm)
    expect(submittedFieldNames(resetForm)).toEqual(['reason'])

    fireEvent.change(field(resetForm, 'reason'), { target: { value: 'phone call verified' } })
    fireEvent.click(resetForm.querySelector('[data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() =>
      expect(vi.mocked(api.resetMemberMfa)).toHaveBeenCalledWith('acct-member', 'phone call verified'),
    )
  })
})
