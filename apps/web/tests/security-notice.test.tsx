import { fireEvent, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SecurityNotice } from '@0xunknownz/foundation-tenant-ui'
import { mockApi, renderWithProviders } from './helpers'

const kinds = [
  { kind: 'credentialsReset' as const, key: 'security.credentialsReset', defaultText: '管理员重置' },
  { kind: 'mfaLocked' as const, key: 'security.mfaLocked', defaultText: 'MFA 连续验证失败' },
  { kind: 'mfaDisabled' as const, key: 'security.mfaDisabled', defaultText: '已停用 MFA' },
]

describe('security messages cannot be removed (TUI-03)', () => {
  it.each(kinds)('$kind: empty and whitespace overrides fall back to the default text', ({ kind, key, defaultText }) => {
    for (const override of ['', '   ', String.fromCharCode(9) + ' ']) {
      const { container, unmount } = renderWithProviders(
        <SecurityNotice kind={kind} onAcknowledge={() => undefined} blocking={false} />,
        { api: mockApi(), messages: { [key]: override } },
      )
      const body = container.querySelector('[data-ftu-security="notice-body"]')
      expect(body?.textContent).toContain(defaultText)
      unmount()
    }
  })

  it.each(kinds)('$kind: a rewritten warning is shown verbatim', ({ kind, key }) => {
    const custom = '请注意：这条提醒已被改写，但仍然必须确认'
    const { container } = renderWithProviders(
      <SecurityNotice kind={kind} onAcknowledge={() => undefined} blocking={false} />,
      { api: mockApi(), messages: { [key]: custom } },
    )
    expect(container.querySelector('[data-ftu-security="notice-body"]')?.textContent).toBe(custom)
  })

  it.each(kinds)('$kind: without an override the default text is shown', ({ kind, defaultText }) => {
    const { container } = renderWithProviders(
      <SecurityNotice kind={kind} onAcknowledge={() => undefined} blocking={false} />,
      { api: mockApi() },
    )
    expect(container.querySelector('[data-ftu-security="notice-body"]')?.textContent).toContain(defaultText)
  })

  it('cannot be acknowledged before the checkbox is ticked', () => {
    const onAcknowledge = vi.fn()
    renderWithProviders(
      <SecurityNotice kind="credentialsReset" onAcknowledge={onAcknowledge} blocking={false} />,
      { api: mockApi() },
    )
    const button = screen.getByRole('button', { name: '我已知晓' })
    expect((button as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(button)
    expect(onAcknowledge).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(button)
    expect(onAcknowledge).toHaveBeenCalledTimes(1)
  })

  it('moves focus into the reminder so keyboard users meet it first', () => {
    renderWithProviders(
      <SecurityNotice kind="mfaLocked" onAcknowledge={() => undefined} blocking={false} />,
      { api: mockApi() },
    )
    expect(document.activeElement).toBe(screen.getByRole('checkbox'))
  })

  it('has no close control and Escape does not dismiss it', () => {
    const onAcknowledge = vi.fn()
    renderWithProviders(
      <SecurityNotice kind="mfaDisabled" onAcknowledge={onAcknowledge} blocking={false} />,
      { api: mockApi() },
    )
    const dialog = screen.getByRole('alertdialog')
    expect(screen.queryByRole('button', { name: /关闭|close|取消|cancel/i })).toBeNull()
    fireEvent.keyDown(dialog, { key: 'Escape' })
    expect(screen.getByRole('alertdialog')).toBeTruthy()
    expect(onAcknowledge).not.toHaveBeenCalled()
  })
})
