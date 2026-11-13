import { fireEvent, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { LoginPage, TenantApiError, showsRequestId } from '@0xunknownz/foundation-tenant-ui'
import { PASSWORD_FIELD, TEST_PW, fieldByName, mockApi, renderWithProviders } from './helpers'

const RID = '0123456789abcdef0123456789abcdef'

function submitLogin(container: HTMLElement) {
  fireEvent.change(fieldByName(container, 'login', 'email'), { target: { value: 'user@acme.test' } })
  fireEvent.change(fieldByName(container, 'login', PASSWORD_FIELD), { target: { value: TEST_PW } })
  fireEvent.click(container.querySelector('form[data-ftu-form="login"] [data-ftu-security="submit"]') as HTMLElement)
}

describe('request id on unexpected errors (TRACE-06)', () => {
  it('a server fault shows the id with a copy action', async () => {
    const api = mockApi()
    vi.mocked(api.login).mockRejectedValueOnce(new TenantApiError(500, 'INTERNAL', 'internal error', RID))
    const { container } = renderWithProviders(<LoginPage />, { api })
    submitLogin(container)
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('请求编号')
    expect(alert.textContent).toContain(RID)
    expect(screen.getByRole('button', { name: '复制' })).toBeTruthy()
  })

  it('a network failure has no id to show', async () => {
    const api = mockApi()
    vi.mocked(api.login).mockRejectedValueOnce(new TenantApiError(0, 'NETWORK_ERROR', 'offline'))
    const { container } = renderWithProviders(<LoginPage />, { api })
    submitLogin(container)
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).not.toContain('请求编号')
  })

  it('a field-level 4xx stays quiet even when the response carried an id', async () => {
    const api = mockApi()
    vi.mocked(api.login).mockRejectedValueOnce(new TenantApiError(401, 'INVALID_CREDENTIALS', 'bad credentials', RID))
    const { container } = renderWithProviders(<LoginPage />, { api })
    submitLogin(container)
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).not.toContain('请求编号')
  })

  it('showsRequestId is limited to unexpected failures with an id', () => {
    const base = { code: 'X', status: 500, message: 'm', requestId: RID }
    expect(showsRequestId({ ...base })).toBe(true)
    expect(showsRequestId({ ...base, status: 0 })).toBe(true)
    expect(showsRequestId({ ...base, code: 'UNKNOWN', status: 400 })).toBe(true)
    expect(showsRequestId({ ...base, status: 422 })).toBe(false)
    expect(showsRequestId({ ...base, requestId: undefined })).toBe(false)
    expect(showsRequestId(null)).toBe(false)
  })
})
