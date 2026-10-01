import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.6-sol'
export const GROK_CC_SWITCH_MODEL = 'grok-4.5'

export type CcSwitchClientType = 'claude' | 'gemini'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
}

/**
 * Balance query CC Switch runs against the imported provider. CC Switch fills
 * `{{baseUrl}}` with the provider's base URL as stored — Grok imports
 * carry a trailing `/v1` (see `withV1Endpoint`), Codex/Claude ones do not, and users
 * may edit it either way afterwards — then evaluates the script, so the URL
 * strips an existing `/v1` instead of blindly appending one (`/v1/v1/usage`
 * is a 404 and CC Switch shows "query failed").
 */
export const CC_SWITCH_USAGE_SCRIPT = `({
    request: {
      url: "{{baseUrl}}".replace(/\\/+$/, "").replace(/\\/v1$/, "") + "/v1/usage",
      method: "GET",
      headers: {
        "Authorization": "Bearer {{apiKey}}",
        "Content-Type": "application/json",
        "User-Agent": "cc-switch/1.0"
      }
    },
    extractor: function(response) {
      if (!response || response.error || response.isValid === false || response.is_active === false) {
        var msg = "API Key 无效或查询失败";
        if (response && response.error && response.error.message) msg = response.error.message;
        else if (response && response.message) msg = response.message;
        return { isValid: false, invalidMessage: msg };
      }
      function pct(used, limit) {
        if (!limit || limit <= 0) return null;
        return Math.round(Math.max(0, Math.min(100, ((used || 0) / limit) * 100)));
      }
      if (response.subscription) {
        var sub = response.subscription;
        var tiers = [];
        var d = pct(sub.daily_usage_usd, sub.daily_limit_usd);
        var w = pct(sub.weekly_usage_usd, sub.weekly_limit_usd);
        var m = pct(sub.monthly_usage_usd, sub.monthly_limit_usd);
        if (d !== null) tiers.push("日 " + d + "%");
        if (w !== null) tiers.push("周 " + w + "%");
        if (m !== null) tiers.push("月 " + m + "%");
        if (response.remaining < 0) {
          return { isValid: true, planName: response.planName || "订阅", remaining: null,
            used: null, total: null, unit: response.unit || "USD", extra: "无限制" };
        }
        var limit = null;
        var used = 0;
        if (sub.monthly_limit_usd > 0) {
          limit = sub.monthly_limit_usd; used = sub.monthly_usage_usd || 0;
        } else if (sub.weekly_limit_usd > 0) {
          limit = sub.weekly_limit_usd; used = sub.weekly_usage_usd || 0;
        } else if (sub.daily_limit_usd > 0) {
          limit = sub.daily_limit_usd; used = sub.daily_usage_usd || 0;
        }
        return { isValid: true, planName: response.planName || "订阅", remaining: response.remaining,
          used: used, total: limit || response.remaining, unit: response.unit || "USD",
          extra: tiers.length > 0 ? tiers.join(" · ") : "无限制" };
      }
      if (response.quota) {
        var q = response.quota;
        var qp = pct(q.used, q.limit);
        return { isValid: true, planName: response.planName || "API Key 配额", remaining: q.remaining,
          used: q.used, total: q.limit, unit: q.unit || "USD", extra: qp !== null ? qp + "%" : "" };
      }
      if (response.rate_limits && response.rate_limits.length > 0) {
        var rls = response.rate_limits;
        var binding = rls[0];
        var bindingRatio = pct(binding.used, binding.limit);
        for (var i = 1; i < rls.length; i++) {
          var r = pct(rls[i].used, rls[i].limit);
          if (r !== null && (bindingRatio === null || r > bindingRatio)) {
            binding = rls[i]; bindingRatio = r;
          }
        }
        var windows = rls.map(function(x) {
          var p = pct(x.used, x.limit);
          return x.window + " " + (p !== null ? p + "%" : "∞");
        }).join(" · ");
        return { isValid: true, planName: response.planName || "速率限制", remaining: binding.remaining,
          used: binding.used, total: binding.limit, unit: "USD", extra: windows };
      }
      var remaining = response.remaining != null ? response.remaining : response.balance;
      var cost = response.usage && response.usage.total ? response.usage.total.cost : 0;
      return {
        isValid: true,
        planName: response.planName || "钱包余额",
        remaining: remaining,
        used: cost || 0,
        total: remaining,
        unit: response.unit || "USD",
        extra: ""
      };
    }
  })`

function withV1Endpoint(baseUrl: string): string {
  const normalizedBaseUrl = baseUrl.replace(/\/+$/, '')
  return normalizedBaseUrl.endsWith('/v1') ? normalizedBaseUrl : `${normalizedBaseUrl}/v1`
}

/**
 * 将脚本以 ASCII 安全的方式 base64 编码。
 *
 * `btoa` 仅接受 Latin1 字符，包含中文等非 ASCII 字符时会抛
 * `InvalidCharacterError`。这里先把每个非 ASCII 字符转成 `\uXXXX` 字面量，
 * 使编码后的源码为纯 ASCII：
 *   - `btoa` 不再抛错；
 *   - CC-Switch 解码后拿到的是合法 JS 源码，JS 引擎会正确还原中文，
 *     与其按 Latin1 还是 UTF-8 解码 base64 字节无关。
 */
function encodeUsageScript(script: string): string {
  let asciiSafe = ''
  for (const ch of script.split('')) {
    const code = ch.charCodeAt(0)
    asciiSafe += code > 0x7f ? `\\u${code.toString(16).padStart(4, '0')}` : ch
  }
  return btoa(asciiSafe)
}

function withoutTrailingSlashes(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, '')
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string
): CcSwitchImportConfig {
  switch (platform || 'anthropic') {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : 'claude',
        endpoint: `${baseUrl.replace(/\/+$/, '')}/antigravity`
      }
    case 'openai':
      return {
        app: 'codex',
        // CC Switch's Codex provider appends the OpenAI-compatible path itself.
        // Passing /v1 here can make the client request /v1/v1/....
        endpoint: withoutTrailingSlashes(baseUrl),
        model: OPENAI_CC_SWITCH_CODEX_MODEL
      }
    case 'gemini':
      return {
        app: 'gemini',
        endpoint: baseUrl
      }
    case 'grok':
      return {
        app: 'grokbuild',
        endpoint: withV1Endpoint(baseUrl),
        model: GROK_CC_SWITCH_MODEL
      }
    default:
      return {
        app: 'claude',
        endpoint: baseUrl
      }
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(input.platform, input.clientType, input.baseUrl)
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', input.baseUrl],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', encodeUsageScript(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  if (config.model) {
    entries.splice(2, 0, ['model', config.model])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
