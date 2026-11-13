import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { BillingPage } from '@0xunknownz/foundation-tenant-ui'
import { billingSubscriptionFixture, mockApi, renderWithProviders } from './helpers'

describe('billing page', () => {
  it('shows the catalogue and purchases a plan', async () => {
    const api = mockApi()
    const { container } = renderWithProviders(<BillingPage navigate={vi.fn()} />, { api })
    await waitFor(() => expect(container.querySelector('[data-ftu-plan="pro"]')).toBeTruthy())
    expect(container.textContent).toContain('专业版')
    expect(container.textContent).toContain('CNY 129.00')
    const plan = container.querySelector('[data-ftu-plan="pro"]') as HTMLElement
    fireEvent.click(within(plan).getByRole('button'))
    await waitFor(() => expect(vi.mocked(api.purchasePlan)).toHaveBeenCalledWith({ planCode: 'pro', interval: 'month', currency: 'CNY' }))
  })

  it('shows the current subscription and cancels it at the period end', async () => {
    const api = mockApi({ billingSubscription: vi.fn(async () => ({ subscription: billingSubscriptionFixture(), provider: 'test' })) })
    const { container } = renderWithProviders(<BillingPage navigate={vi.fn()} />, { api })
    await waitFor(() => expect(container.querySelector('[data-ftu-billing-subscription]')).toBeTruthy())
    expect(container.querySelector('[data-ftu-billing-plan]')?.textContent).toBe('pro')
    expect(container.querySelector('[data-ftu-billing-status]')?.textContent).toBe('已生效')
    fireEvent.click(screen.getByRole('button', { name: '取消订阅' }))
    await waitFor(() => expect(vi.mocked(api.cancelSubscription)).toHaveBeenCalled())
  })

  it('disables purchase when no payment provider is configured', async () => {
    const api = mockApi({
      listBillingPlans: vi.fn(async () => ({ items: [{ code: 'pro', name: '专业版', description: '', features: [], limits: {}, prices: [{ interval: 'month' as const, currency: 'CNY', amountCents: 100 }] }], provider: 'none' })),
      billingSubscription: vi.fn(async () => ({ subscription: null, provider: 'none' })),
    })
    const { container } = renderWithProviders(<BillingPage navigate={vi.fn()} />, { api })
    await waitFor(() => expect(container.querySelector('[data-ftu-plan="pro"]')).toBeTruthy())
    const plan = container.querySelector('[data-ftu-plan="pro"]') as HTMLElement
    expect((within(plan).getByRole('button') as HTMLButtonElement).disabled).toBe(true)
    expect(container.textContent).toContain('未配置支付提供方')
  })
})
