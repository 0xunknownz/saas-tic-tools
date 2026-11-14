import { cleanup, fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { TenantApiError } from '@0xunknownz/foundation-tenant-ui'
import { billingSubscriptionFixture, mockApi, renderWithProviders } from '../../web/tests/helpers'
import { BillingRoute } from '../src/routes/billing'

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () { this.open = true }
  HTMLDialogElement.prototype.close = function () { this.open = false }
})
afterEach(cleanup)
const catalogue = { provider: 'test', items: [{ code: 'pro', name: '专业版', description: '实际接口描述', features: ['advanced'], limits: { users: 10 }, prices: [{ interval: 'month' as const, currency: 'CNY', amountCents: 12901 }, { interval: 'year' as const, currency: 'USD', amountCents: 9900 }] }] }

describe('TIC billing administration', () => {
  it('keeps integer cents, interval and currency paired with every purchase; never fabricates missing prices', async () => {
    const api = mockApi({ listBillingPlans: vi.fn(async () => catalogue) })
    renderWithProviders(<BillingRoute/>, { api })
    await screen.findByText('专业版')
    expect(screen.getByText('CNY 129.01')).toBeTruthy()
    expect(screen.getByText('USD 99.00')).toBeTruthy()
    expect(screen.getByText('users：10')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: '购买专业版，年付，USD' }))
    await screen.findByText(/购买请求已受理/)
    expect(api.purchasePlan).toHaveBeenLastCalledWith({ planCode: 'pro', interval: 'year', currency: 'USD' })
    fireEvent.click(screen.getByRole('button', { name: '购买专业版，月付，CNY' }))
    await waitFor(() => expect(api.purchasePlan).toHaveBeenCalledTimes(2))
    expect(api.purchasePlan).toHaveBeenLastCalledWith({ planCode: 'pro', interval: 'month', currency: 'CNY' })
    cleanup()
    renderWithProviders(<BillingRoute/>, { api: mockApi() })
    expect(await screen.findByText('未提供年付价格')).toBeTruthy()
    expect(screen.queryByRole('button', { name: /年付/ })).toBeNull()
  })

  it('disables purchases with no provider while retaining allowed cancellation', async () => {
    const api = mockApi({ listBillingPlans: vi.fn(async () => catalogue), billingSubscription: vi.fn(async () => ({ provider: 'none', subscription: billingSubscriptionFixture() })) })
    renderWithProviders(<BillingRoute/>, { api })
    await screen.findByText(/未配置支付提供方/)
    screen.getAllByRole('button', { name: /购买专业版/ }).forEach(button => {
      expect((button as HTMLButtonElement).disabled).toBe(true)
      fireEvent.click(button)
    })
    expect(api.purchasePlan).not.toHaveBeenCalled()
    expect((screen.getByRole('button', { name: '取消订阅' }) as HTMLButtonElement).disabled).toBe(false)
  })

  it('keeps failed purchases retryable and prevents duplicate requests during checkout', async () => {
    let finish: () => void = () => undefined
    const api = mockApi({ purchasePlan: vi.fn().mockRejectedValueOnce(new TenantApiError(503, 'TEMPORARY', '购买失败，请重试')).mockImplementationOnce(() => new Promise(resolve => { finish = () => resolve({ subscriptionId: 'sub-1', provider: 'test', reference: 'checkout-1', amountCents: 12900, currency: 'CNY' }) })) })
    renderWithProviders(<BillingRoute/>, { api })
    const buy = await screen.findByRole('button', { name: '购买专业版，月付，CNY' })
    fireEvent.click(buy)
    await screen.findByText('购买失败，请重试')
    expect(screen.queryByText(/购买请求已受理/)).toBeNull()
    fireEvent.click(buy); fireEvent.click(buy)
    expect((buy as HTMLButtonElement).disabled).toBe(true)
    expect(api.purchasePlan).toHaveBeenCalledTimes(2)
    finish()
    await screen.findByText(/购买请求已受理/)
  })

  it('confirms cancellation, keeps failures honest and refreshes actual period-end state', async () => {
    let current = billingSubscriptionFixture()
    const api = mockApi({ billingSubscription: vi.fn(async () => ({ subscription: current, provider: 'test' })), cancelSubscription: vi.fn().mockRejectedValueOnce(new TenantApiError(503, 'TEMPORARY', '取消失败')).mockImplementationOnce(async () => { current = { ...current, cancelAtPeriodEnd: true }; return current }) })
    renderWithProviders(<BillingRoute/>, { api })
    fireEvent.click(await screen.findByRole('button', { name: '取消订阅' }))
    fireEvent.click(screen.getByRole('button', { name: '保留订阅' }))
    expect(api.cancelSubscription).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: '取消订阅' }))
    fireEvent.click(screen.getByRole('button', { name: '确认取消订阅' }))
    await screen.findByRole('dialog')
    await waitFor(() => expect(screen.getAllByText('取消失败').length).toBeGreaterThan(0))
    expect(screen.queryByText('到期取消')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '确认取消订阅' }))
    await screen.findByText('到期取消')
    expect(screen.getByText(/当前已付服务可使用至 2026-12-01 00:00 UTC/)).toBeTruthy()
    expect(screen.queryByRole('button', { name: '取消订阅' })).toBeNull()
    expect(api.cancelSubscription).toHaveBeenCalledTimes(2)
    expect(api.billingSubscription).toHaveBeenCalledTimes(2)
  })

  it('uses actual status rather than plan name and handles absent expiry, subscription and catalogue', async () => {
    renderWithProviders(<BillingRoute/>, { api: mockApi({ billingSubscription: vi.fn(async () => ({ provider: 'test', subscription: billingSubscriptionFixture({ status: 'canceled', periodEnd: null }) })) }) })
    await screen.findByText('已取消')
    expect(screen.getByText('未提供')).toBeTruthy()
    expect(screen.queryByText('当前套餐')).toBeNull()
    expect(screen.queryByRole('button', { name: '取消订阅' })).toBeNull()
    cleanup()
    renderWithProviders(<BillingRoute/>, { api: mockApi({ listBillingPlans: vi.fn(async () => ({ provider: 'test', items: [] })) }) })
    expect(await screen.findByText('当前没有订阅，可在下方选择套餐。')).toBeTruthy()
    expect(screen.getByText('暂无可用套餐。')).toBeTruthy()
  })

  it('explains pending cancellation and refuses to claim refreshed success when reload fails', async () => {
    const api = mockApi({ billingSubscription: vi.fn().mockResolvedValue({ provider: 'test', subscription: null }).mockResolvedValueOnce({ provider: 'test', subscription: billingSubscriptionFixture({ status: 'pending', periodEnd: null }) }).mockRejectedValueOnce(new TenantApiError(503, 'TEMPORARY', '重新读取失败')) })
    renderWithProviders(<BillingRoute/>, { api })
    await screen.findByText('待支付')
    expect(screen.queryByText('当前套餐')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '取消订阅' }))
    expect(screen.getByText(/待支付订阅将终止/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: '确认取消订阅' }))
    expect(await screen.findByText('订阅与套餐加载失败，请重试。')).toBeTruthy()
    expect(screen.queryByText('到期取消')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '重新加载账单' }))
    await screen.findByText('当前没有订阅，可在下方选择套餐。')
  })
})
