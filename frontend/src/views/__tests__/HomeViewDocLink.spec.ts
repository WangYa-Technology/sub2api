import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

describe('HomeView documentation link', () => {
  it('uses the public doc_url and removes the fallback link when unset', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/views/HomeView.vue'), 'utf8')

    expect(source).toContain('sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || \'\')')
    expect(source).toContain('v-if="docUrl"')
    expect(source).toContain(':href="docUrl"')
  })
})
