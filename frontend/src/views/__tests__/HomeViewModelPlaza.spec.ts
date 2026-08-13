import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

describe('HomeView model plaza entry', () => {
  it('adds the entry only when the plaza is public', () => {
    const source = readFileSync(
      resolve(process.cwd(), 'src/views/HomeView.vue'),
      'utf8'
    )

    expect(source).toContain('settings?.model_plaza_enabled')
    expect(source).toContain('settings.model_plaza_require_auth')
    expect(source).toContain("entry.href = '/model-plaza'")
    expect(source).toContain("entry.textContent = '模型广场'")
    expect(source).toContain('navCta.before(entry)')
  })
})
