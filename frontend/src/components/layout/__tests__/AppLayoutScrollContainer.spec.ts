import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const layoutSource = readFileSync(resolve(dir, '../AppLayout.vue'), 'utf8')
const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')
const globalStyleSource = readFileSync(resolve(dir, '../../../style.css'), 'utf8')
const settingsSource = readFileSync(resolve(dir, '../../../views/admin/SettingsView.vue'), 'utf8')

describe('AppLayout scroll ownership', () => {
  it('keeps document scrolling out of the application shell', () => {
    expect(layoutSource).toContain('class="h-screen overflow-hidden bg-gray-50 dark:bg-dark-950"')
    expect(layoutSource).toContain('class="app-main-scroll p-4 md:p-6 lg:p-8"')
    expect(layoutSource).toContain('@apply min-h-0 flex-1 overflow-y-auto;')
    expect(layoutSource).toContain('scrollbar-gutter: stable;')
  })

  it('keeps the header outside the main scroll container', () => {
    expect(headerSource).toContain('sticky top-0 z-30 shrink-0')
    expect(layoutSource.indexOf('<AppHeader />')).toBeLessThan(
      layoutSource.indexOf('<main class="app-main-scroll')
    )
  })

  it('anchors settings tabs to the main scroll container without a second header offset', () => {
    const settingsTabsRule = settingsSource.match(/\.settings-tabs-shell\s*\{[\s\S]*?\n\}/)

    expect(settingsTabsRule).not.toBeNull()
    expect(settingsTabsRule?.[0]).toContain('top: 0;')
    expect(settingsTabsRule?.[0]).not.toContain('top: 4.75rem;')
  })

  it('does not rely on the root scrollbar gutter workaround', () => {
    const htmlBlock = globalStyleSource.match(/html\s*\{[\s\S]*?\n {2}\}/)

    expect(htmlBlock).not.toBeNull()
    expect(htmlBlock?.[0]).not.toContain('scrollbar-gutter')
  })
})
