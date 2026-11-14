import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { TenantApiError } from '@0xunknownz/foundation-tenant-ui'
import { TicForgotPasswordPage } from '../src/features/auth/TicForgotPasswordPage'
import { mockApi, renderWithProviders } from './helpers'

const sentMessage = '如果该邮箱对应一个账号，重置链接已发送。请查收邮件。'

describe('TIC password reset request', () => {
  it.each([['', '请输入账号邮箱。'], ['invalid-address', '请输入有效的邮箱地址']])('associates email validation with the input: %s', async (value, message) => {
    const api = mockApi()
    renderWithProviders(<TicForgotPasswordPage navigate={vi.fn()} />, { api })
    const input = screen.getByRole('textbox', { name: '邮箱' })
    fireEvent.change(input, { target: { value } })
    fireEvent.click(screen.getByRole('button', { name: '发送重置链接' }))
    expect(screen.getByRole('alert').textContent).toContain(message)
    expect(input.getAttribute('aria-invalid')).toBe('true')
    expect(document.getElementById(input.getAttribute('aria-describedby')!)?.textContent).toContain(message)
    expect(document.activeElement).toBe(input)
    expect(api.requestPasswordReset).not.toHaveBeenCalled()
  })
  it('prevents repeated submissions while pending and retains email for a failed request retry', async () => {
    let reject!: (error: Error) => void
    const api = mockApi({ requestPasswordReset: vi.fn(() => new Promise((_, fail) => { reject = fail })) })
    renderWithProviders(<TicForgotPasswordPage navigate={vi.fn()} />, { api })
    const input = screen.getByRole('textbox', { name: '邮箱' }) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'person@example.test' } })
    const form = screen.getByRole('form', { name: '找回密码' })
    fireEvent.submit(form)
    fireEvent.submit(form)
    expect(api.requestPasswordReset).toHaveBeenCalledTimes(1)
    expect((screen.getByRole('button', { name: '发送中…' }) as HTMLButtonElement).disabled).toBe(true)
    await act(async () => reject(new TenantApiError(0, 'NETWORK_ERROR', 'offline')))
    expect((await screen.findByRole('alert')).textContent).toContain('请求失败')
    expect(input.value).toBe('person@example.test')
    vi.mocked(api.requestPasswordReset).mockResolvedValue({ sent: true })
    fireEvent.click(screen.getByRole('button', { name: '发送重置链接' }))
    expect((await screen.findByRole('status')).textContent).toBe(sentMessage)
    expect(screen.queryByRole('form')).toBeNull()
  })
  it.each([true, false])('shows identical conditional confirmation for response sent=%s', async sent => {
    const api = mockApi({ requestPasswordReset: vi.fn(async () => ({ sent })) })
    const navigate = vi.fn()
    renderWithProviders(<TicForgotPasswordPage navigate={navigate} />, { api })
    fireEvent.change(screen.getByRole('textbox', { name: '邮箱' }), { target: { value: 'person@example.test' } })
    fireEvent.click(screen.getByRole('button', { name: '发送重置链接' }))
    expect((await screen.findByRole('status')).textContent).toBe(sentMessage)
    expect(api.requestPasswordReset).toHaveBeenCalledWith('person@example.test')
    fireEvent.click(screen.getByRole('link', { name: '返回登录' }))
    expect(navigate).toHaveBeenCalledWith('/login')
  })
  it('returns to login before a submission without requesting a reset', () => {
    const api = mockApi(), navigate = vi.fn()
    renderWithProviders(<TicForgotPasswordPage navigate={navigate} />, { api })
    fireEvent.click(screen.getByRole('link', { name: '返回登录' }))
    expect(navigate).toHaveBeenCalledWith('/login')
    expect(api.requestPasswordReset).not.toHaveBeenCalled()
  })
})
