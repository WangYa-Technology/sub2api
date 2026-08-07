/**
 * Shared URL builder for iframe-embedded pages.
 * Used by PurchaseSubscriptionView and CustomPageView to build consistent URLs
 * with user_id, token, theme, lang, ui_mode, src_host, and src parameters.
 */

const EMBEDDED_USER_ID_QUERY_KEY = 'user_id'
const EMBEDDED_AUTH_TOKEN_QUERY_KEY = 'token'
const EMBEDDED_THEME_QUERY_KEY = 'theme'
const EMBEDDED_LANG_QUERY_KEY = 'lang'
const EMBEDDED_UI_MODE_QUERY_KEY = 'ui_mode'
const EMBEDDED_UI_MODE_VALUE = 'embedded'
const EMBEDDED_SRC_HOST_QUERY_KEY = 'src_host'
const EMBEDDED_SRC_QUERY_KEY = 'src_url'

export function buildEmbeddedUrl(
  baseUrl: string,
  userId?: number,
  authToken?: string | null,
  theme: 'light' | 'dark' = 'light',
  lang?: string,
  options?: { includeCredentials?: boolean },
): string {
  if (!baseUrl) return baseUrl
  const includeCredentials = options?.includeCredentials ?? true
  try {
    const url = new URL(baseUrl)
    if (includeCredentials) {
      if (userId) {
        url.searchParams.set(EMBEDDED_USER_ID_QUERY_KEY, String(userId))
      }
      if (authToken) {
        url.searchParams.set(EMBEDDED_AUTH_TOKEN_QUERY_KEY, authToken)
      }
      // Source tracking: let the embedded page know where it's being loaded from.
      // These are only attached for trusted (same-origin) targets; a cross-origin
      // page can read its own query string, so credentials and the current page
      // URL must never be sent to an untrusted origin.
      if (typeof window !== 'undefined') {
        url.searchParams.set(EMBEDDED_SRC_HOST_QUERY_KEY, window.location.origin)
        url.searchParams.set(EMBEDDED_SRC_QUERY_KEY, window.location.href)
      }
    }
    url.searchParams.set(EMBEDDED_THEME_QUERY_KEY, theme)
    if (lang) {
      url.searchParams.set(EMBEDDED_LANG_QUERY_KEY, lang)
    }
    url.searchParams.set(EMBEDDED_UI_MODE_QUERY_KEY, EMBEDDED_UI_MODE_VALUE)
    return url.toString()
  } catch {
    return baseUrl
  }
}

/**
 * Returns true when the target URL is trusted enough to receive session
 * credentials in its query string: same-origin with the host app, or a
 * relative path (which resolves against the host origin). Absolute URLs on
 * other origins (including protocol-relative //host) are never trusted.
 */
export function isSameOriginEmbedTarget(raw: string): boolean {
  if (!raw) return false
  const trimmed = raw.trim()
  if (!trimmed) return false
  // Relative paths resolve against the host origin; treat them as same-origin.
  if (!/^[a-z][a-z0-9+.-]*:/i.test(trimmed) && !trimmed.startsWith('//')) {
    return true
  }
  try {
    return new URL(trimmed).origin === window.location.origin
  } catch {
    return false
  }
}

/**
 * Builds a URL for sidebar "open in new window" (external) menu items.
 *
 * Unlike buildEmbeddedUrl, this NEVER attaches the user id, auth token, source
 * host or current page URL. The target may be an arbitrary third-party origin
 * outside our control; embedding session credentials or the current page URL
 * would leak them to that origin via the address bar, browser history, Referer
 * headers and server logs.
 *
 * Only plain http(s) URLs are accepted; anything else returns an empty string
 * so the caller can treat the item as not external. theme/lang are intentionally
 * omitted as well: an external tab is not an embedded page.
 */
export function buildExternalUrl(baseUrl: string): string {
  if (!baseUrl) return ''
  if (!baseUrl.startsWith('http://') && !baseUrl.startsWith('https://')) {
    return ''
  }
  try {
    return new URL(baseUrl).toString()
  } catch {
    return ''
  }
}

export function detectTheme(): 'light' | 'dark' {
  if (typeof document === 'undefined') return 'light'
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}
