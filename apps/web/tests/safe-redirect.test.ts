import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { redirectTo, safeRedirect } from '@0xunknownz/foundation-tenant-ui'

const BS = String.fromCharCode(92)

describe('safeRedirect (TUI-02)', () => {
  let warn: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)
  })

  afterEach(() => {
    warn.mockRestore()
  })

  it('keeps same-site relative paths, including query and hash', () => {
    const accepted = [
      '/dashboard',
      '/reports?month=11#top',
      '/',
      '/members?q=%E5%BC%A0',
      '/%E6%88%90%E5%91%98',
    ]
    for (const target of accepted) {
      expect(safeRedirect(target, '/')).toBe(target)
    }
    expect(warn).not.toHaveBeenCalled()
  })

  it('falls back for at least twelve malicious targets', () => {
    const malicious: unknown[] = [
      'https://evil.example',
      'http://evil.example/steal',
      '//evil.example',
      '///evil.example',
      'javascript:alert(1)',
      'data:text/html,alert(1)',
      '/' + BS + 'evil.example',
      BS + BS + 'evil.example',
      '/%5Cevil.example',
      '/%2F%2Fevil.example',
      '/ok' + String.fromCharCode(10) + 'Set-Cookie:x',
      '/%0d%0aLocation:%20https://evil.example',
      'dashboard',
      '',
      ' https://evil.example',
      '/%2e%2e/%2e%2e/evil',
      '/%zz',
      undefined,
      null,
    ]
    expect(malicious.length).toBeGreaterThanOrEqual(12)
    for (const target of malicious) {
      expect(safeRedirect(target, '/')).toBe('/')
    }
    for (const target of malicious) {
      if (target === '' || target === undefined || target === null) continue
      expect(warn).toHaveBeenCalled()
    }
  })

  it('uses a custom fallback', () => {
    expect(safeRedirect('//evil.example', '/login')).toBe('/login')
    expect(safeRedirect(undefined, '/login')).toBe('/login')
  })

  it('warns once per rejected target and never navigates off-site', () => {
    warn.mockClear()
    safeRedirect('https://evil.example')
    expect(warn).toHaveBeenCalledTimes(1)
    expect(String(warn.mock.calls[0]?.[0])).toContain('[tenant-ui]')
  })

  it('applies the same check from redirectTo', () => {
    const navigate = vi.fn()
    expect(redirectTo(navigate, '//evil.example')).toBe('/')
    expect(navigate).toHaveBeenCalledWith('/')
    expect(redirectTo(navigate, '/reports?month=11#top')).toBe('/reports?month=11#top')
    expect(navigate).toHaveBeenLastCalledWith('/reports?month=11#top')
  })
})
