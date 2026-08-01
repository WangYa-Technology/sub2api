import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppHeader model plaza entry', () => {
  it('renders as a featured navigation entry with an active state', () => {
    expect(componentSource).toContain('class="model-plaza-entry group hidden sm:flex"')
    expect(componentSource).toContain("'model-plaza-entry-active': route.path === '/model-plaza'")
    expect(componentSource).toContain('<Icon name="grid"')
    expect(componentSource).toContain('.model-plaza-entry {')
    expect(componentSource).toContain('.model-plaza-entry-active {')
  })
})
