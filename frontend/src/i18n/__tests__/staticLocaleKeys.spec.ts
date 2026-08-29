import { readdirSync, readFileSync } from 'node:fs'
import { extname, join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type Messages = Record<string, unknown>

const sourceRoot = join(process.cwd(), 'src')

function flattenKeys(node: Messages, prefix = ''): string[] {
  const keys: string[] = []
  for (const [key, value] of Object.entries(node)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      keys.push(...flattenKeys(value as Messages, path))
    } else {
      keys.push(path)
    }
  }
  return keys
}

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) {
      return entry.name === '__tests__' ? [] : sourceFiles(path)
    }
    return ['.ts', '.vue'].includes(extname(entry.name)) && !entry.name.endsWith('.spec.ts') ? [path] : []
  })
}

function staticKeyReferences(): Map<string, Set<string>> {
  const references = new Map<string, Set<string>>()
  const patterns = [
    /(?:\bi18n(?:\.global)?\.t|\bt|\$t)\(\s*(['"`])([A-Za-z0-9_.-]+)\1/g,
    /(?:titleKey|descriptionKey)\s*:\s*(['"`])([A-Za-z0-9_.-]+)\1/g
  ]

  for (const file of sourceFiles(sourceRoot)) {
    const content = readFileSync(file, 'utf8')
    for (const pattern of patterns) {
      for (const match of content.matchAll(pattern)) {
        const key = match[2]
        if (key.endsWith('.')) continue
        const locations = references.get(key) ?? new Set<string>()
        locations.add(relative(sourceRoot, file))
        references.set(key, locations)
      }
    }
  }
  return references
}

describe('locale key integrity', () => {
  const enKeys = new Set(flattenKeys(en))
  const zhKeys = new Set(flattenKeys(zh))

  it('keeps English and Chinese locale structures in sync', () => {
    expect([...enKeys].filter((key) => !zhKeys.has(key)).sort(), 'missing from zh').toEqual([])
    expect([...zhKeys].filter((key) => !enKeys.has(key)).sort(), 'missing from en').toEqual([])
  })

  it('defines every statically referenced translation key in both locales', () => {
    const missing = [...staticKeyReferences()]
      .filter(([key]) => !enKeys.has(key) || !zhKeys.has(key))
      .map(([key, files]) => `${key} (${[...files].sort().join(', ')})`)
      .sort()

    expect(missing).toEqual([])
  })
})
