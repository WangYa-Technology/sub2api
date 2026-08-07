import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { buildEmbeddedUrl, buildExternalUrl, detectTheme, isSameOriginEmbedTarget } from '../embedded-url'

describe('embedded-url', () => {
  const originalLocation = window.location

  beforeEach(() => {
    Object.defineProperty(window, 'location', {
      value: {
        origin: 'https://app.example.com',
        href: 'https://app.example.com/user/purchase',
      },
      writable: true,
      configurable: true,
    })
  })

  afterEach(() => {
    Object.defineProperty(window, 'location', {
      value: originalLocation,
      writable: true,
      configurable: true,
    })
    document.documentElement.classList.remove('dark')
    vi.restoreAllMocks()
  })

  it('adds embedded query parameters including locale and source context', () => {
    const result = buildEmbeddedUrl(
      'https://pay.example.com/checkout?plan=pro',
      42,
      'token-123',
      'dark',
      'zh-CN',
    )

    const url = new URL(result)
    expect(url.searchParams.get('plan')).toBe('pro')
    expect(url.searchParams.get('user_id')).toBe('42')
    expect(url.searchParams.get('token')).toBe('token-123')
    expect(url.searchParams.get('theme')).toBe('dark')
    expect(url.searchParams.get('lang')).toBe('zh-CN')
    expect(url.searchParams.get('ui_mode')).toBe('embedded')
    expect(url.searchParams.get('src_host')).toBe('https://app.example.com')
    expect(url.searchParams.get('src_url')).toBe('https://app.example.com/user/purchase')
  })

  it('omits optional params when they are empty', () => {
    const result = buildEmbeddedUrl('https://pay.example.com/checkout', undefined, '', 'light')

    const url = new URL(result)
    expect(url.searchParams.get('theme')).toBe('light')
    expect(url.searchParams.get('ui_mode')).toBe('embedded')
    expect(url.searchParams.has('user_id')).toBe(false)
    expect(url.searchParams.has('token')).toBe(false)
    expect(url.searchParams.has('lang')).toBe(false)
  })

  it('returns original string for invalid url input', () => {
    expect(buildEmbeddedUrl('not a url', 1, 'token')).toBe('not a url')
  })

  it('omits credentials and source context when includeCredentials is false', () => {
    const result = buildEmbeddedUrl(
      'https://pay.example.com/checkout',
      42,
      'token-123',
      'dark',
      'zh-CN',
      { includeCredentials: false },
    )

    const url = new URL(result)
    expect(url.searchParams.has('user_id')).toBe(false)
    expect(url.searchParams.has('token')).toBe(false)
    expect(url.searchParams.has('src_host')).toBe(false)
    expect(url.searchParams.has('src_url')).toBe(false)
    // Non-sensitive params stay intact for the embedded page.
    expect(url.searchParams.get('theme')).toBe('dark')
    expect(url.searchParams.get('lang')).toBe('zh-CN')
    expect(url.searchParams.get('ui_mode')).toBe('embedded')
  })

  it('detects dark mode from document root class', () => {
    document.documentElement.classList.add('dark')
    expect(detectTheme()).toBe('dark')
  })
})

describe('isSameOriginEmbedTarget', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'location', {
      value: { origin: 'https://app.example.com' },
      writable: true,
      configurable: true,
    })
  })

  it('trusts same-origin absolute urls and relative paths', () => {
    expect(isSameOriginEmbedTarget('https://app.example.com/pay')).toBe(true)
    expect(isSameOriginEmbedTarget('/pay')).toBe(true)
    expect(isSameOriginEmbedTarget('pages/status')).toBe(true)
  })

  it('rejects cross-origin, protocol-relative and invalid urls', () => {
    expect(isSameOriginEmbedTarget('https://evil.example.com/pay')).toBe(false)
    expect(isSameOriginEmbedTarget('//evil.example.com/pay')).toBe(false)
    expect(isSameOriginEmbedTarget('javascript:alert(1)')).toBe(false)
    expect(isSameOriginEmbedTarget('')).toBe(false)
    expect(isSameOriginEmbedTarget(undefined as unknown as string)).toBe(false)
  })
})

describe('buildExternalUrl', () => {
  it('returns the plain URL without any credentials or source context', () => {
    const result = buildExternalUrl('https://docs.example.com/guide?lang=en')
    expect(result).toBe('https://docs.example.com/guide?lang=en')
    expect(result).not.toContain('token')
    expect(result).not.toContain('user_id')
    expect(result).not.toContain('src_url')
    expect(result).not.toContain('ui_mode')
  })

  it('accepts http and https only', () => {
    expect(buildExternalUrl('http://example.com/')).toBe('http://example.com/')
    expect(buildExternalUrl('https://example.com/')).toBe('https://example.com/')
  })

  it('rejects non-http schemes, javascript: and invalid input', () => {
    expect(buildExternalUrl('javascript:alert(1)')).toBe('')
    expect(buildExternalUrl('file:///etc/passwd')).toBe('')
    expect(buildExternalUrl('ftp://example.com/')).toBe('')
    expect(buildExternalUrl('not a url')).toBe('')
    expect(buildExternalUrl('')).toBe('')
    expect(buildExternalUrl(undefined as unknown as string)).toBe('')
  })
})
