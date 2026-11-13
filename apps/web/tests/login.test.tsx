import { fireEvent, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { TenantApiError } from '@0xunknownz/foundation-tenant-ui'
import { LOGIN_SLOTS, LoginPage } from '@0xunknownz/foundation-tenant-ui'
import { MfaEnrollPage } from '@0xunknownz/foundation-tenant-ui'
import { MfaVerifyPage } from '@0xunknownz/foundation-tenant-ui'
import { CURRENT_PW, PASSWORD_FIELD, TEST_PW, fieldByName, mockApi, renderWithProviders } from './helpers'

function fill(container: HTMLElement, formName: string, values: Record<string, string>): void {
  for (const [name, value] of Object.entries(values)) {
    fireEvent.change(fieldByName(container, formName, name), { target: { value } })
  }
}

describe('login flow and redirects (TUI-02)', () => {
  it('allows a retry after a network failure', async () => {
    const api = mockApi()
    vi.mocked(api.login).mockRejectedValueOnce(new TenantApiError(0, 'NETWORK_ERROR', 'offline'))
    const navigate = vi.fn()
    const { container } = renderWithProviders(<LoginPage navigate={navigate} />, { api })

    fill(container, 'login', { email: 'user@acme.test', [PASSWORD_FIELD]: TEST_PW })
    fireEvent.click(container.querySelector('form[data-ftu-form="login"] [data-ftu-security="submit"]') as HTMLElement)
    expect((await screen.findByRole('alert')).textContent).toContain('登录请求失败')
    const submit = container.querySelector('form[data-ftu-form="login"] [data-ftu-security="submit"]') as HTMLButtonElement
    expect(submit.disabled).toBe(false)

    fireEvent.click(submit)
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/'))
  })

  it('honours same-site redirects and rejects off-site targets from onSuccess', async () => {
    for (const [wanted, expected] of [
      ['/dashboard', '/dashboard'],
      ['/reports?month=11#top', '/reports?month=11#top'],
      ['//evil.example', '/'],
      ['https://evil.example', '/'],
    ] as const) {
      const api = mockApi()
      const navigate = vi.fn()
      const view = renderWithProviders(
        <LoginPage onSuccess={() => wanted} navigate={navigate} />,
        { api },
      )
      fill(view.container, 'login', { email: 'user@acme.test', [PASSWORD_FIELD]: TEST_PW })
      fireEvent.click(view.container.querySelector('[data-ftu-form="login"] [data-ftu-security="submit"]') as HTMLElement)
      await waitFor(() => expect(navigate).toHaveBeenCalledWith(expected))
      view.unmount()
    }
  })

  it('applies the same check to the redirect query parameter', async () => {
    const backslash = String.fromCharCode(92)
    const cases = [
      ['/dashboard', '/dashboard'],
      ['/reports?month=11#top', '/reports?month=11#top'],
      ['https://evil.example', '/'],
      ['//evil.example', '/'],
      ['javascript:alert(1)', '/'],
      ['/' + backslash + 'evil.example', '/'],
    ] as const
    for (const [wanted, expected] of cases) {
      const api = mockApi()
      const navigate = vi.fn()
      const view = renderWithProviders(<LoginPage redirect={wanted} navigate={navigate} />, { api })
      fill(view.container, 'login', { email: 'user@acme.test', [PASSWORD_FIELD]: TEST_PW })
      fireEvent.click(view.container.querySelector('[data-ftu-form="login"] [data-ftu-security="submit"]') as HTMLElement)
      await waitFor(() => expect(navigate).toHaveBeenCalledWith(expected))
      view.unmount()
    }
  })

  it('routes restricted sessions to the right next step', async () => {
    const cases = [
      ['mfa_pending', '/mfa/verify'],
      ['setup', '/mfa/enroll'],
    ] as const
    for (const [level, expected] of cases) {
      const api = mockApi({ login: vi.fn(async () => ({ email: 'user@acme.test', level })) })
      const navigate = vi.fn()
      const view = renderWithProviders(<LoginPage navigate={navigate} />, { api })
      fill(view.container, 'login', { email: 'user@acme.test', [PASSWORD_FIELD]: TEST_PW })
      fireEvent.click(view.container.querySelector('[data-ftu-form="login"] [data-ftu-security="submit"]') as HTMLElement)
      await waitFor(() => expect(navigate).toHaveBeenCalledWith(expected))
      view.unmount()
    }
  })

  it('shows a blocking security reminder before continuing after a credential reset', async () => {
    const api = mockApi({
      login: vi.fn(async () => ({ email: 'user@acme.test', level: 'setup' as const, notice: 'password_reset' })),
    })
    const navigate = vi.fn()
    const { container } = renderWithProviders(<LoginPage slots={{ footer: null }} navigate={navigate} />, { api })
    fill(container, 'login', { email: 'user@acme.test', [PASSWORD_FIELD]: TEST_PW })
    fireEvent.click(container.querySelector('[data-ftu-form="login"] [data-ftu-security="submit"]') as HTMLElement)

    await waitFor(() => expect(screen.getByRole('alertdialog')).toBeTruthy())
    expect(container.querySelector('[data-ftu-form="login"]')).toBeNull()
    expect(screen.getByRole('alertdialog').textContent).toContain('管理员重置')

    const acknowledge = screen.getByRole('button', { name: '我已知晓' })
    fireEvent.click(acknowledge)
    expect(navigate).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(acknowledge)
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/mfa/enroll'))
  })

  it('marks the OIDC entry as not enabled', () => {
    const api = mockApi()
    const { container } = renderWithProviders(<LoginPage navigate={vi.fn()} />, { api })
    const mark = container.querySelector('[data-ftu-oidc="disabled"]')
    expect(mark?.textContent).toContain('尚未启用')
    expect(mark?.getAttribute('aria-disabled')).toBe('true')
  })
})

describe('MFA pages', () => {
  it('completes second-factor verification and redirects safely', async () => {
    const api = mockApi()
    const navigate = vi.fn()
    const { container } = renderWithProviders(
      <MfaVerifyPage redirect="/reports?month=11#top" navigate={navigate} />,
      { api },
    )
    fill(container, 'mfa-verify', { code: '123456' })
    fireEvent.click(container.querySelector('[data-ftu-form="mfa-verify"] [data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(vi.mocked(api.verifyMfa)).toHaveBeenCalledWith('123456'))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/reports?month=11#top'))
  })

  it('shows recovery codes once and requires saving them before continuing', async () => {
    const api = mockApi()
    const navigate = vi.fn()
    const { container } = renderWithProviders(<MfaEnrollPage navigate={navigate} />, { api })
    fill(container, 'mfa-enroll-password', { [PASSWORD_FIELD]: CURRENT_PW })
    fireEvent.click(container.querySelector('form[data-ftu-form="mfa-enroll-password"] [data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(container.querySelector('[data-ftu-mfa-secret]')).toBeTruthy())

    fill(container, 'mfa-enroll-code', { code: '123456' })
    fireEvent.click(container.querySelector('form[data-ftu-form="mfa-enroll-code"] [data-ftu-security="submit"]') as HTMLElement)
    await waitFor(() => expect(container.querySelector('[data-ftu-recovery-codes]')).toBeTruthy())
    expect(container.querySelectorAll('[data-ftu-recovery-codes] li').length).toBe(10)

    const continueButton = container.querySelector('[data-ftu-security="recovery-continue"]') as HTMLButtonElement
    expect(continueButton.disabled).toBe(true)
    fireEvent.click(container.querySelector('[data-ftu-recovery-saved]') as HTMLElement)
    fireEvent.click(continueButton)
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/'))
  })
})

describe('slot constants', () => {
  it('documents the public slot names', () => {
    expect(LOGIN_SLOTS).toEqual(['header', 'aside', 'beforeForm', 'afterForm', 'footer'])
  })
})
