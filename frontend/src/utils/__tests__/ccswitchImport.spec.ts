import { describe, expect, it } from 'vitest'
import {
  CC_SWITCH_USAGE_SCRIPT,
  GROK_CC_SWITCH_MODEL,
  OPENAI_CC_SWITCH_CODEX_MODEL,
  buildCcSwitchImportDeeplink
} from '@/utils/ccswitchImport'
import type { GroupPlatform } from '@/types'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

describe('ccswitchImport utils', () => {
  it('defaults OpenAI CC Switch imports to the current Codex model', () => {
    expect(OPENAI_CC_SWITCH_CODEX_MODEL).toBe('gpt-5.6-sol')
  })

  it('defaults Grok Build imports to the current Grok model', () => {
    expect(GROK_CC_SWITCH_MODEL).toBe('grok-4.5')
  })

  const baseInput = {
    baseUrl: 'https://api.example.com',
    providerName: 'Sub2API',
    apiKey: 'sk-test',
    usageScript: 'return true'
  }

  it.each([
    ['https://api.example.com', 'https://api.example.com'],
    ['https://api.example.com/', 'https://api.example.com'],
    ['https://api.example.com/v1', 'https://api.example.com/v1'],
    ['https://api.example.com/v1/', 'https://api.example.com/v1']
  ])('keeps Codex imports on the configured endpoint for base URL %s', (baseUrl, endpoint) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        baseUrl,
        platform: 'openai',
        clientType: 'claude'
      })
    )

    expect(params.get('resource')).toBe('provider')
    expect(params.get('app')).toBe('codex')
    expect(params.get('endpoint')).toBe(endpoint)
    expect(params.get('model')).toBe(OPENAI_CC_SWITCH_CODEX_MODEL)
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
  })

  it.each([
    'https://api.example.com',
    'https://api.example.com/',
    'https://api.example.com/v1',
    'https://api.example.com/v1/'
  ])('imports Grok Build with one /v1 suffix for base URL %s', (baseUrl) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        baseUrl,
        platform: 'grok',
        clientType: 'claude'
      })
    )

    expect(params.get('app')).toBe('grokbuild')
    expect(params.get('endpoint')).toBe('https://api.example.com/v1')
    expect(params.get('model')).toBe(GROK_CC_SWITCH_MODEL)
  })

  it.each([
    { platform: 'anthropic' as GroupPlatform, clientType: 'claude' as const, app: 'claude' },
    { platform: 'gemini' as GroupPlatform, clientType: 'gemini' as const, app: 'gemini' }
  ])('does not add a model parameter for $platform imports', ({ platform, clientType, app }) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform,
        clientType
      })
    )

    expect(params.get('app')).toBe(app)
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.has('model')).toBe(false)
  })

  it('keeps Antigravity imports on the selected client endpoint without a model parameter', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'antigravity',
        clientType: 'gemini'
      })
    )

    expect(params.get('app')).toBe('gemini')
    expect(params.get('endpoint')).toBe(`${baseInput.baseUrl}/antigravity`)
    expect(params.has('model')).toBe(false)
  })

  it('encodes usage scripts with non-Latin1 characters without throwing and round-trips back to a JS-equivalent source', () => {
    const chineseScript = 'function (r) { return { planName: "订阅" }; }'

    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        usageScript: chineseScript
      })
    )

    // btoa 在遇到非 Latin1 字符时会抛 InvalidCharacterError；这里只要能拿到值就说明没抛错。
    const encoded = params.get('usageScript')
    expect(encoded).toBeTruthy()
    // 解码后是纯 ASCII 的合法 JS 源码，中文被转成 \uXXXX 字面量，与原脚本在 JS 语义上等价。
    const decoded = atob(encoded!)
    expect(decoded).not.toContain('订阅')
    expect(decoded).toContain('\\u8ba2\\u9605')
    // eslint-disable-next-line no-new-func
    const fn = new Function('return (' + decoded + ')') as () => (r: unknown) => { planName: string }
    expect(fn()({}).planName).toBe('订阅')
  })
})

describe('CC Switch usage script', () => {
  function extractUsage(response: unknown): Record<string, unknown> {
    const params = paramsFromDeeplink(buildCcSwitchImportDeeplink({
      baseUrl: 'https://api.example.com', platform: 'openai', clientType: 'claude',
      providerName: 'HCAI', apiKey: 'sk-test', usageScript: CC_SWITCH_USAGE_SCRIPT
    }))
    // Evaluate the actual ASCII-safe payload received by CC Switch.
    // eslint-disable-next-line no-new-func
    const config = new Function(`return ${atob(params.get('usageScript')!)}`)() as {
      extractor: (value: unknown) => Record<string, unknown>
    }
    return config.extractor(response)
  }

  it('preserves subscription windows and zero usage in the selected window', () => {
    expect(extractUsage({ remaining: 20, planName: 'Pro', subscription: {
      daily_usage_usd: 2, daily_limit_usd: 10,
      weekly_usage_usd: 5, weekly_limit_usd: 20,
      monthly_usage_usd: 0, monthly_limit_usd: 100
    } })).toMatchObject({ isValid: true, planName: 'Pro', used: 0, total: 100,
      remaining: 20, extra: '日 20% · 周 25% · 月 0%' })
    expect(extractUsage({ remaining: -1, subscription: {} })).toMatchObject({
      remaining: null, used: null, total: null, extra: '无限制'
    })
  })

  it('preserves quota and chooses the most-used rate window', () => {
    expect(extractUsage({ quota: { used: 25, limit: 100, remaining: 75 } })).toMatchObject({
      planName: 'API Key 配额', used: 25, total: 100, remaining: 75, extra: '25%'
    })
    expect(extractUsage({ rate_limits: [
      { window: '5h', used: 10, limit: 100, remaining: 90 },
      { window: '7d', used: 80, limit: 100, remaining: 20 }
    ] })).toMatchObject({ planName: '速率限制', used: 80, total: 100, remaining: 20,
      extra: '5h 10% · 7d 80%' })
  })

  it('preserves balance usage and rejects inactive or failed responses', () => {
    expect(extractUsage({ balance: 30, usage: { total: { cost: 12 } } })).toMatchObject({
      isValid: true, planName: '钱包余额', remaining: 30, total: 30, used: 12, unit: 'USD'
    })
    expect(extractUsage({ error: { message: 'expired' } })).toEqual({
      isValid: false, invalidMessage: 'expired'
    })
    for (const response of [null, { is_active: false }, { isValid: false }]) {
      expect(extractUsage(response).isValid).toBe(false)
    }
  })

  // Mirrors CC Switch: substitute the template vars as text, evaluate, read request.url.
  function usageUrlFor(baseUrl: string): string {
    const script = CC_SWITCH_USAGE_SCRIPT.split('{{baseUrl}}').join(baseUrl).split('{{apiKey}}').join('sk-test')
    // eslint-disable-next-line no-new-func
    const config = new Function(`return ${script}`)() as { request: { url: string } }
    return config.request.url
  }

  it.each([
    'https://api.example.com',
    'https://api.example.com/',
    'https://api.example.com/v1',
    'https://api.example.com/v1/'
  ])('queries exactly one /v1/usage for base URL %s', (baseUrl) => {
    expect(usageUrlFor(baseUrl)).toBe('https://api.example.com/v1/usage')
  })

  it('works against the endpoint every platform import stores', () => {
    for (const platform of ['anthropic', 'openai', 'grok', 'gemini'] as GroupPlatform[]) {
      const endpoint = paramsFromDeeplink(
        buildCcSwitchImportDeeplink({
          baseUrl: 'https://api.example.com',
          platform,
          clientType: platform === 'gemini' ? 'gemini' : 'claude',
          providerName: 'Sub2API',
          apiKey: 'sk-test',
          usageScript: CC_SWITCH_USAGE_SCRIPT
        })
      ).get('endpoint') as string
      expect(usageUrlFor(endpoint)).toBe('https://api.example.com/v1/usage')
    }
  })
})
