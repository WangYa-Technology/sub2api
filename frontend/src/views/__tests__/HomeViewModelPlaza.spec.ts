import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

describe('HomeView model plaza entry', () => {
  it('adds the entry only when the plaza is public', () => {
    const source = readFileSync(
      resolve(process.cwd(), 'src/views/HomeView.vue'),
      'utf8'
    )

    expect(source).toContain('showModelPlazaEntry')
    expect(source).toContain('model_plaza_require_auth')
    expect(source).toContain('to="/model-plaza"')
  })
})
