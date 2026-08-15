export const THEME_STORAGE_KEY = 'theme'
export const THEME_CHANGE_EVENT = 'hcai:theme-change'

export type Theme = 'light' | 'dark'

export interface ThemeChangeDetail {
  theme: Theme
}

function getStoredTheme(): Theme | null {
  const theme = localStorage.getItem(THEME_STORAGE_KEY)
  return theme === 'light' || theme === 'dark' ? theme : null
}

function getSystemTheme(mediaQuery: MediaQueryList): Theme {
  return mediaQuery.matches ? 'dark' : 'light'
}

export function isDarkTheme(): boolean {
  return document.documentElement.classList.contains('dark')
}

export function applyTheme(theme: Theme): void {
  const root = document.documentElement
  root.classList.toggle('dark', theme === 'dark')
  root.style.colorScheme = theme
  document.dispatchEvent(
    new CustomEvent<ThemeChangeDetail>(THEME_CHANGE_EVENT, {
      detail: { theme }
    })
  )
}

export function setTheme(theme: Theme): void {
  localStorage.setItem(THEME_STORAGE_KEY, theme)
  applyTheme(theme)
}

export function initializeTheme(): () => void {
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')

  const syncTheme = () => {
    applyTheme(getStoredTheme() ?? getSystemTheme(mediaQuery))
  }

  const handleSystemThemeChange = () => {
    if (!getStoredTheme()) {
      applyTheme(getSystemTheme(mediaQuery))
    }
  }

  syncTheme()
  mediaQuery.addEventListener('change', handleSystemThemeChange)

  return () => {
    mediaQuery.removeEventListener('change', handleSystemThemeChange)
  }
}
