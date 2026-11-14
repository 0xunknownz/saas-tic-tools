import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { TenantApiError, type LoginResult } from '@0xunknownz/foundation-tenant-ui'
import { TicLoginPage } from '../src/features/auth/TicLoginPage'
import { TicTenantSelectPage } from '../src/features/auth/TicTenantSelectPage'
import { adminSession, fieldByName, mockApi, renderWithProviders, TEST_PW } from './helpers'

const memberships = [
  { id: 'tenant-1', slug: 'acme', name: '验收机构甲', status: 'active', role: 'tenant_admin' },
  { id: 'tenant-2', slug: 'beta', name: '验收机构乙长组织名称', status: 'active', role: 'member' },
]
function listedApi() {
  return mockApi({ listTenants: vi.fn(async () => ({ items: memberships, nextCursor: null, total: 2 })) })
}
function fillLogin(container: HTMLElement) {
  fireEvent.change(fieldByName(container, 'login', 'email'), { target: { value: 'owner@acme.test' } })
  fireEvent.change(fieldByName(container, 'login', 'password'), { target: { value: TEST_PW } })
}

describe('TIC login entry', () => {
  it('uses the branded form, blocks repeated pending submits and retries a visible failure', async () => {
    let reject!: (error: Error) => void
    const api = mockApi({ login: vi.fn(() => new Promise<LoginResult>((_, fail) => { reject = fail })) })
    const navigate = vi.fn()
    const { container } = renderWithProviders(<TicLoginPage navigate={navigate} />, { api })
    expect(screen.getByRole('heading', { name: '欢迎回来' })).toBeTruthy()
    fillLogin(container)
    fireEvent.click(screen.getByRole('button', { name: '登录', exact: true }))
    fireEvent.submit(container.querySelector('form')!)
    expect(api.login).toHaveBeenCalledTimes(1)
    await act(async () => reject(new TenantApiError(0, 'NETWORK_ERROR', 'offline')))
    expect((await screen.findByRole('alert')).textContent).toContain('登录请求失败')
    vi.mocked(api.login).mockResolvedValue({ email: 'owner@acme.test', level: 'full' })
    fireEvent.click(screen.getByRole('button', { name: '登录', exact: true }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/'))
  })
  it('keeps credential reset confirmation blocking and forget-password working', async () => {
    const api = mockApi({ login: vi.fn(async () => ({ email: 'owner@acme.test', level: 'setup' as const, notice: 'password_reset' })) })
    const navigate = vi.fn()
    const { container } = renderWithProviders(<TicLoginPage navigate={navigate} />, { api })
    fireEvent.click(screen.getByRole('button', { name: '忘记密码？' }))
    expect(navigate).toHaveBeenCalledWith('/forgot-password')
    navigate.mockClear()
    fillLogin(container)
    fireEvent.click(screen.getByRole('button', { name: '登录', exact: true }))
    await screen.findByRole('alertdialog')
    expect((screen.getByRole('button', { name: '我已知晓' }) as HTMLButtonElement).disabled).toBe(true)
    expect(navigate).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(screen.getByRole('button', { name: '我已知晓' }))
    expect(navigate).toHaveBeenCalledWith('/mfa/enroll')
  })
})

describe('TIC tenant entry', () => {
  it('shows the real account, membership roles and current tenant, and directly enters one row', async () => {
    const api = listedApi(), navigate = vi.fn()
    renderWithProviders(<TicTenantSelectPage navigate={navigate} redirect="/projects/project-001?tab=summary#details" />, { api })
    expect(screen.getByRole('status').textContent).toContain('正在加载')
    const enter = await screen.findByRole('button', { name: '进入 验收机构乙长组织名称' })
    expect(screen.getByText('owner@acme.test')).toBeTruthy()
    expect(screen.getByText('管理员')).toBeTruthy()
    expect(screen.getByText('成员')).toBeTruthy()
    expect(screen.getByText('当前租户').closest('li')?.textContent).toContain('验收机构甲')
    fireEvent.click(enter)
    await waitFor(() => expect(api.switchTenant).toHaveBeenCalledWith('tenant-2'))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/projects/project-001?tab=summary#details'))
  })
  it('disables all actions while entering and stays on errors for retry', async () => {
    let reject!: (error: Error) => void
    const api = listedApi(), navigate = vi.fn()
    vi.mocked(api.switchTenant).mockImplementationOnce(() => new Promise((_, fail) => { reject = fail }))
    renderWithProviders(<TicTenantSelectPage navigate={navigate} />, { api })
    const enter = await screen.findByRole('button', { name: '进入 验收机构甲' })
    fireEvent.click(enter)
    fireEvent.click(enter)
    expect(api.switchTenant).toHaveBeenCalledTimes(1)
    expect((screen.getByRole('button', { name: '使用其他账号' }) as HTMLButtonElement).disabled).toBe(true)
    await act(async () => reject(new TenantApiError(503, 'UNAVAILABLE', 'offline')))
    await screen.findByRole('alert')
    expect(navigate).not.toHaveBeenCalled()
    fireEvent.click(enter)
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/'))
  })
  it('retries session and list failures without pretending the account has no tenants', async () => {
    const api = listedApi()
    vi.mocked(api.session).mockRejectedValueOnce(new TenantApiError(503, 'UNAVAILABLE', 'offline'))
    vi.mocked(api.listTenants).mockRejectedValueOnce(new TenantApiError(503, 'UNAVAILABLE', 'offline')).mockRejectedValueOnce(new TenantApiError(503, 'UNAVAILABLE', 'offline'))
    renderWithProviders(<TicTenantSelectPage navigate={vi.fn()} />, { api })
    fireEvent.click(await screen.findByRole('button', { name: '重新加载' }))
    const retry = await screen.findByRole('button', { name: '重试加载组织' })
    expect(screen.queryByText('尚未加入任何租户')).toBeNull()
    expect(screen.queryByRole('textbox', { name: '租户 ID' })).toBeNull()
    fireEvent.click(retry)
    await screen.findByRole('button', { name: '进入 验收机构甲' })
  })
  it('shows an empty membership state and supports reloading it', async () => {
    const api = mockApi({ session: vi.fn(async () => adminSession({ tenant: null, role: null })) })
    renderWithProviders(<TicTenantSelectPage navigate={vi.fn()} />, { api })
    await screen.findByText('尚未加入任何租户')
    vi.mocked(api.listTenants).mockResolvedValue({ items: memberships, nextCursor: null, total: 2 })
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }))
    await screen.findByRole('button', { name: '进入 验收机构甲' })
  })
  it.each([404, 405, 501])('keeps the manual ID fallback for unavailable list endpoint %s', async status => {
    const api = listedApi(), navigate = vi.fn()
    vi.mocked(api.listTenants).mockRejectedValue(new TenantApiError(status, 'HTTP_' + status, 'unavailable'))
    renderWithProviders(<TicTenantSelectPage navigate={navigate} />, { api })
    fireEvent.change(await screen.findByRole('textbox', { name: '租户 ID' }), { target: { value: ' tenant-9 ' } })
    fireEvent.click(screen.getByRole('button', { name: '进入工作空间' }))
    await waitFor(() => expect(api.switchTenant).toHaveBeenCalledWith('tenant-9'))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/'))
  })
  it('waits for genuine logout, shows failure and retries before returning to login', async () => {
    const api = listedApi(), navigate = vi.fn()
    vi.mocked(api.logout).mockRejectedValueOnce(new TenantApiError(0, 'NETWORK_ERROR', 'offline'))
    renderWithProviders(<TicTenantSelectPage navigate={navigate} redirect="/projects/project-001#details" />, { api })
    fireEvent.click(await screen.findByRole('button', { name: '使用其他账号' }))
    expect((await screen.findByRole('alert')).textContent).toContain('退出失败')
    expect(navigate).not.toHaveBeenCalled()
    expect(screen.getByText('owner@acme.test')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: '使用其他账号' }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/login?redirect=' + encodeURIComponent('/projects/project-001#details')))
    expect(api.logout).toHaveBeenCalledTimes(2)
  })
  it.each(['session', 'list', 'switch'])('preserves a safe target on %s expiry', async phase => {
    const api = listedApi(), navigate = vi.fn()
    const expired = new TenantApiError(401, 'SESSION_EXPIRED', 'expired')
    if (phase === 'session') vi.mocked(api.session).mockRejectedValue(expired)
    if (phase === 'list') vi.mocked(api.listTenants).mockRejectedValue(expired)
    if (phase === 'switch') vi.mocked(api.switchTenant).mockRejectedValue(expired)
    renderWithProviders(<TicTenantSelectPage navigate={navigate} redirect="/projects/project-001?tab=a#details" />, { api })
    if (phase === 'switch') fireEvent.click(await screen.findByRole('button', { name: '进入 验收机构甲' }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/login?redirect=' + encodeURIComponent('/projects/project-001?tab=a#details')))
  })
  it('rejects a normalized external target when expired', async () => {
    const api = listedApi(), navigate = vi.fn()
    vi.mocked(api.session).mockRejectedValue(new TenantApiError(401, 'UNAUTHENTICATED', 'expired'))
    renderWithProviders(<TicTenantSelectPage navigate={navigate} redirect="/console/..//evil.example/landing" />, { api })
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/login?redirect=%2F'))
  })
})
