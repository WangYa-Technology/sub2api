import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  initializeTheme,
  setTheme,
  THEME_CHANGE_EVENT,
  THEME_STORAGE_KEY
} from '@/utils/theme'

describe('theme preference', () => {
  let systemDark = false
  let systemChangeHandler: (() => void) | undefined

  beforeEach(() => {
    systemDark = false
    systemChangeHandler = undefined
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.style.colorScheme = ''

    vi.stubGlobal('matchMedia', vi.fn(() => ({
      get matches() {
        return systemDark
      },
      media: '(prefers-color-scheme: dark)',
      onchange: null,
      addEventListener: vi.fn((_type: string, handler: () => void) => {
        systemChangeHandler = handler
      }),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn()
    })))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.style.colorScheme = ''
  })

  it('follows system changes until the user chooses a theme', () => {
    systemDark = true
    initializeTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBeNull()

    systemDark = false
    systemChangeHandler?.()

    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('keeps a saved theme when the system changes', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'light')
    systemDark = true
    initializeTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(false)

    systemDark = false
    systemChangeHandler?.()
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('persists manual changes and announces the applied theme', () => {
    const listener = vi.fn()
    document.addEventListener(THEME_CHANGE_EVENT, listener)

    setTheme('dark')

    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.style.colorScheme).toBe('dark')
    expect(listener).toHaveBeenCalledOnce()

    document.removeEventListener(THEME_CHANGE_EVENT, listener)
  })
})
