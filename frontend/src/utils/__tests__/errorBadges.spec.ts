import { describe, expect, it } from 'vitest'

import {
  numericRequestTypeKind,
  requestTypeBadgeClass,
  requestTypeLabelKey,
} from '../errorBadges'

describe('error request type badges', () => {
  it('maps persisted request type numbers without conflicting with Live', () => {
    expect(numericRequestTypeKind(4)).toBe('cyber')
    expect(numericRequestTypeKind(5)).toBe('live')
    expect(numericRequestTypeKind(6)).toBe('async')
  })

  it('provides the async label and badge color', () => {
    expect(requestTypeLabelKey('async')).toBe('usage.async')
    expect(requestTypeBadgeClass('async')).toContain('bg-cyan-100')
  })
})
