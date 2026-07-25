import { describe, expect, it } from 'vitest'

import {
  isUsageRequestType,
  requestTypeToLegacyStream,
  resolveUsageRequestType,
} from '../usageRequestType'

describe('usageRequestType async support', () => {
  it('recognizes and preserves async request types', () => {
    expect(isUsageRequestType('async')).toBe(true)
    expect(resolveUsageRequestType({ request_type: 'async', stream: false })).toBe('async')
  })

  it('maps async to the non-streaming legacy field', () => {
    expect(requestTypeToLegacyStream('async')).toBe(false)
  })
})
