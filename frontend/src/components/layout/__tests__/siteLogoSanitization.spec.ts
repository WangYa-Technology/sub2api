import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const sidebarSource = readFileSync(resolve(dir, '../AppSidebar.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')
const hcaiHomeScriptSource = readFileSync(resolve(dir, '../../../../public/hcai/main.js'), 'utf8')

describe('site_logo sanitization', () => {
  it('AppSidebar imports sanitizeUrl and applies it to siteLogo', () => {
    expect(sidebarSource).toContain("import { sanitizeUrl } from '@/utils/url'")
    expect(sidebarSource).toContain('sanitizeUrl(appStore.siteLogo')
  })

  it('HCAI home sanitizes the public site logo before using it as a favicon', () => {
    expect(hcaiHomeScriptSource).toContain('updateFavicon(sanitizeImageUrl(settings && settings.site_logo))')
    expect(hcaiHomeScriptSource).toContain('trimmed.startsWith("data:image/")')
    expect(hcaiHomeScriptSource).toContain('protocol !== "http:" && protocol !== "https:"')
  })

  it('KeyUsageView applies sanitizeUrl to siteLogo', () => {
    expect(keyUsageViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  it('Vue-rendered logos pass allowRelative and allowDataUrl options', () => {
    for (const src of [sidebarSource, keyUsageViewSource]) {
      expect(src).toContain('allowRelative: true')
      expect(src).toContain('allowDataUrl: true')
    }
  })
})
