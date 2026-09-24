# HCAI 自定义功能与内容清单

> 0.2.8-hcai 新基线：本次保留全部 HCAI 定制，并新增上游 TypeSafe/OpenCode/推理强度计费/线下提现能力；详细冲突和验证见 [上游合并记录-0.2.8-hcai](上游合并记录-0.2.8-hcai.md)。旧版基线快照不回写。

> 版本基线：`0.2.7-hcai`（当前 `dev` 分支，提交 `9345eab94`；`main` 合并提交 `4a118044`）。
>
> 本文档用于回答“哪些内容是 HCAI 自定义、如何生效、在哪里维护、与上游有什么边界”。盘点依据为当前代码、数据库迁移、前端静态资源、测试、HCAI 提交历史以及 v0.2.7 合并记录。上游同步后仍保留在 HCAI 分支中的能力也单独标注，避免把所有上游功能误认为 HCAI 原创。

## 1. 分类口径

| 标记 | 含义 |
| --- | --- |
| HCAI 专属 | HCAI 产品、品牌、业务流程或部署策略直接新增的能力 |
| HCAI 改造 | 上游已有能力被 HCAI 修改了行为、优先级、安全边界或展示方式 |
| HCAI 保留 | 上游同步时明确保留、修复或重新接线的能力；实现可能来自上游，但属于 HCAI 版本必须持续保护的范围 |
| 上游能力 | 当前版本存在但没有足够证据表明是 HCAI 定制，不在本文档作为 HCAI 功能承诺 |

## 2. 总览

| 模块 | 定制内容 | 主要入口 |
| --- | --- | --- |
| 品牌首页 | HCAI 独立静态首页、模型/优势/动态能力、主题、文档链接、模型广场入口、加载态 | `frontend/public/hcai/`、`frontend/src/views/HomeView.vue` |
| 模型广场 | 双币种、充值汇率、官方价/实收价/折扣、渠道模型价格优先且只影响展示、完整中英文文案 | `frontend/src/components/modelPlaza/`、`backend/internal/service/model_pricing_resolver.go` |
| 插件平台 | 插件管理开关持久化、插件 KV、沙箱 UI、Bridge Token、管理 API | `backend/internal/service/plugin_manager.go`、`backend/pkg/pluginapi/` |
| 自定义菜单 | iframe/外链、新窗口模式、分组菜单、外链凭证隔离与 URL 清洗 | `frontend/src/components/layout/AppSidebar.vue`、`frontend/src/utils/embedded-url.ts` |
| CC Switch | OAuth 回调桥接、桌面端回调、四种用量计费模式导入 | `frontend/src/views/auth/OAuthCallbackView.vue`、`frontend/src/utils/ccswitchImport.ts` |
| 支付与返利 | 内置支付、支付宝沙箱、兑换码销售价、按来源返利和审计 | `backend/internal/payment/`、`backend/internal/service/affiliate_service.go` |
| 运维告警 | 企业微信告警、规则/事件通知、去重清理、多节点聚合 | `backend/internal/service/ops_wecom_notification.go`、`backend/internal/service/ops_alert_evaluator_service.go` |
| 多区域运行 | leader lock、OAuth 刷新租约、任务协调、节点/区域指标、跨节点缓存失效 | `backend/internal/service/leader_lock.go`、`backend/internal/repository/concurrency_cache.go` |
| 风控审核 | 内容审核节点、额外审核端点、节点快照与集群聚合 | `backend/internal/service/content_moderation.go`、`frontend/src/views/admin/RiskControlView.vue` |
| 图片任务 | 异步图片任务、事件/错误/用量记录、批量图片与媒体兼容 | `backend/internal/service/image_task.go`、`backend/internal/handler/async_image_ops.go` |
| 网关兼容 | Claude Code URL、Responses reasoning ID、Antigravity/Gemini/OpenAI/图片等 HCAI 修复 | `backend/internal/service/`、`backend/internal/pkg/` |
| 发布部署 | `-hcai` 版本规则、Linux amd64 包、checksum/BUILD_INFO、HCAI CI | `.github/workflows/amd-linux-release.yml`、`backend/scripts/resolve-version.sh` |

## 3. HCAI 首页与品牌资源

### 3.1 页面内容

本站首页是一套独立静态资源：

- 品牌名、Logo、favicon、中文定位文案和 HCAI-CHAT 内测内容。
- Hero 区、代码接入示例、模型展示、优势区、动态能力区、价格表和页脚。
- 模型广场入口；根据公开设置和是否需要登录动态显示。
- 公共文档链接；站点 Logo 使用安全 URL 处理，禁止任意协议注入。
- 系统主题和用户主题偏好同步到首页。
- Three.js 行星/动态视觉；3D 资源不可用时降级为无 3D 页面。

### 3.2 加载链路

`HomeView.vue` 使用 `loading -> ready | failed` 状态：

1. 先请求公开设置，判断 compact home、模型广场、文档链接等开关。
2. 请求 `/hcai/page.html`，插入页面 DOM。
3. 等待 `/hcai/style.css`，HTML 和样式就绪后切换到 HCAI 页面，再插入 `/hcai/main.js`。不会等待 Three.js 动画初始化完成。
4. 等待阶段显示 Logo 和三个脉冲圆点，包含无障碍加载文案和减少动画适配，避免正常加载时先显示默认首页。
5. HTML/样式失败时内部状态变为 `failed`，记录控制台警告，模板实际回退默认首页；目前没有独立可见错误页。
6. 后台 `home_content`、`compact_home_enabled` 仍可选择其他首页形态；HCAI 页面不是无条件覆盖后台设置。

### 3.3 资源清单

- `frontend/public/hcai/page.html`
- `frontend/public/hcai/style.css`
- `frontend/public/hcai/main.js`
- `frontend/public/hcai/planets.js`
- `frontend/public/hcai/three.core.min.js`
- `frontend/public/hcai/three.module.min.js`
- `frontend/public/hcai/hcai-logo.svg`
- `frontend/public/hcai/logo.png`
- `frontend/public/hcai/assets/image-2-showcase-CRhrZOEM.png`

对应测试包括首页加载态、资源失败、模型广场入口、文档链接和 Logo 清洗：
`frontend/src/views/__tests__/HomeView.compact.spec.ts`、`HomeViewDocLink.spec.ts`、`HomeViewModelPlaza.spec.ts`、`frontend/src/components/layout/__tests__/siteLogoSanitization.spec.ts`。

## 4. 模型广场与价格展示

### 4.1 设置项

| 设置键 | 用途 | 默认值 |
| --- | --- | --- |
| `model_plaza_enabled` | 是否开启模型广场 | `false` |
| `model_plaza_require_auth` | 是否要求登录 | `false` |
| `model_plaza_description` | 顶部说明 Markdown | 空 |
| `model_plaza_cny_per_usd` | 充值人民币/美元换算汇率 | 由 `ModelPlazaCNYPerUSDDefault` 提供 |

其中 充值人民币/美元换算汇率 `model_plaza_cny_per_usd` 由 HCAI 设置，本项作为模型广场“人民币实收价”列的换算倍率。只影响展示形式，不影响实际计费逻辑。

实现位置：`backend/internal/service/domain_constants.go`、`setting_parse.go`、`setting_public.go`、`setting_update.go`、`frontend/src/views/admin/SettingsView.vue`。

### 4.2 定价解析链路

### 4.2.1 官方价格数据来源

模型广场后半列的“官方价格”不是从渠道配置读取，也不是从前端静态写死。`ModelPlazaService.lookupOfficialPricing` 调用 `BillingService.GetModelPricing(model)`，使用与实际计费同源的价格目录：

1. LiteLLM/服务端价格目录中的模型价格；
2. 目录缺失时使用 `billing_service.go` 内置 fallback 价格卡；
3. 对能解析的模型再附加官方长上下文阶梯和缓存档位。

价格单位是 USD per token，前端按每百万 token、请求或图片单位格式化；人民币展示使用 `model_plaza_cny_per_usd`。因此，修改渠道模型价格会改变“实收价格”列，不会覆盖官方参考价列；修改官方目录/fallback 才会改变官方列。

模型广场展示调用 `ModelPricingResolver.Resolve` 时设置 `PreferChannelPricing=true`：

```text
渠道中对当前分组/模型的显式价格
    ↓ 未命中
分组模型价格
    ↓ 未命中
渠道覆盖价格 + LiteLLM/官方基础价
    ↓ 未命中
LiteLLM 价格目录
    ↓ 未命中
内置 fallback 价格
```

HCAI 特殊规则：

- 渠道模型价格在模型广场展示链路中优先于分组和官方价。
- 该优先级只改变展示，不改变真实请求的计费候选和扣费逻辑。
- 渠道优先分支直接调用 `GetChannelModelPricing`；普通渠道解析另有 OpenAI/Codex 基名归一化回退，不能假定两条分支完全等价。
- Token、按次、图片、视频、缓存读写、长上下文阶梯、分时价格和 reasoning multiplier 均可展示。
- 配置了渠道价格的字段直接覆盖；未配置字段按解析器规则处理，不把展示价误当成实际扣费价。

核心实现：`backend/internal/service/model_pricing_resolver.go`、`model_plaza_service.go`、`billing_context_schedule.go`、`backend/internal/handler/model_plaza_handler.go`。

### 4.3 前端展示

`PlazaModelPricingTable.vue` 展示：

- 实收价格（人民币）和官方价格（美元/官方目录口径）。
- CNY/USD 换算后的参考倍率、折扣和帮助提示。
- 每百万 token、每次请求、每张图片等单位文案。
- 输入、输出、缓存写入、缓存读取、长上下文阶梯和分时段行。
- 中英文完整键集，避免显示 `modelPlaza.table.unitPerMillion` 之类未翻译 key。

相关测试：`frontend/src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts`、`PlazaGroupSection.spec.ts`、`backend/internal/service/model_pricing_resolver_test.go`、`setting_model_plaza_test.go`。

## 5. 插件平台与插件管理

### 2026-09-20 修订：0.2.7-hcai.1 宿主兼容基线

插件普通版本范围按 HCAI 对应上游基线校验：仅 `X.Y.Z-hcai` 和 `X.Y.Z-hcai.N`（N 为合法数字修订号）映射为 `X.Y.Z`。显式包含预发布/HCAI 后缀的约束仍按原始版本比较；rc、beta 及未知后缀不升级为正式版。协议版本检查不变，对外显示完整 HCAI 版本。

插件声明仅测试过上游版本时，HCAI 版本仍标记为“范围兼容、未经声明测试”，不能冒充已测试；管理员确认机制保留。实现和边界测试位于 `plugin_compatibility.go`、`plugin_compatibility_test.go`。详见 [修复与部署记录](插件兼容性修复与部署-0.2.7-hcai.1.md)。本段为新增修订，非旧版历史行为。

### 5.1 开关持久化

`plugin_management_enabled` 是 DB-backed 设置：

```text
SettingsView.vue
  → PUT 管理设置 API
  → setting_handler_update.go
  → updates[SettingKeyPluginManagementEnabled]
  → SettingRepository 持久化
  → PublicSettings / featureFlags
  → 路由、侧边栏和插件管理页同时生效
```

默认关闭；后端和前端都 fail-closed。此处是 v0.1.183 合并后曾出现的关键回归点，`e633ed510`、`790474868`、`efb70b1b2` 专门恢复了持久化和 HCAI 首页。

### 5.2 插件运行时

- 插件包、manifest、版本和 artifact 由 `backend/migrations/229_plugins.sql`、`230_plugin_artifacts.sql` 支持。
- 插件管理器负责安装、启停、路由和生命周期。
- 插件 KV 存储由 `backend/internal/repository/plugin_kv_store.go` 提供。
- 插件 UI 运行在仅 `allow-scripts` 的 sandbox iframe 中，不直接获得管理员 Token。
- 宿主发放短时资源 URL 和 Bridge Token；Bridge 同时校验消息来源窗口和 Token。
- API、UI bridge、manifest 和包格式文档位于 `backend/pkg/pluginapi/docs/`。

## 6. 自定义菜单、外链和嵌入安全

自定义菜单存储在 `custom_menu_items` JSON 数组，支持：

- 菜单分组和分组内项目；分组不会被错误渲染为独立链接。
- `iframe` 和 `external` 两种打开方式。
- `open_mode=external` 新窗口打开；兼容上游 `hide_open_button` 字段。
- 外链 URL 清洗和嵌入参数隔离，禁止把 `user_id`、JWT、来源上下文拼到第三方 URL。
- iframe 页面使用独立路由和安全的嵌入策略。

关键文件：`AppSidebar.vue`、`AppHeader.vue`、`frontend/src/utils/embedded-url.ts`、`embedded-url.spec.ts`、`docUrlSanitization.spec.ts`。修复记录见 `a07a6f9e2`。

## 7. HCAI Switch 与桌面端集成

### 7.1 OAuth 回调

- 支持 `ccswitch://hcai/oauth/callback` 自定义协议。
- 浏览器回调失败时支持桌面端 callback fallback。
- 对 desktop OAuth redirect 做 URL 解码和来源/target 校验。
- 将 access token、refresh token、过期时间和 API 地址交给 HCAI Switch。

实现和测试：`frontend/src/views/auth/OAuthCallbackView.vue`、`OAuthCallbackView.spec.ts`、中英文 `common.ts`。

### 7.2 用量导入

`ccswitchImport.ts` 的 usage script 支持四种计费模式：

1. `subscription`：日/周/月窗口和无限订阅。
2. `quota-limited`：已用、总量和百分比。
3. `rate-limited`：以最高使用率窗口为主指标，其余窗口放入 `extra`。
4. `balance`：从 `usage.total.cost` 计算余额用量。

同时透传 `planName`、`used`、`total`、`remaining`、`extra`，识别 `error` 和 `isValid=false`，并使用兼容旧 JS 的语法及明确的 `User-Agent`/`Content-Type`。测试：`frontend/src/utils/__tests__/ccswitchImport.spec.ts`。

## 8. 支付、兑换码与邀请返利

### 8.1 支付

> 本站未启用内支付能力，本修改仅作为测试使用

EasyPay、官方支付宝、官方微信支付、Stripe、Airwallex 是当前上游支付基础能力。已确认的 HCAI 支付差异是支付宝沙箱环境：状态按 provider 保存，进行中的订单不能切换网关。移动端预创建/Deep Link 属于本版本保留的支付能力，不认定为 HCAI 原创。

入口：`backend/internal/payment/provider/alipay.go`、`payment_config_providers.go`、`frontend/src/components/payment/PaymentProviderDialog.vue`、`frontend/src/views/user/PaymentView.vue`。配置和回调说明见 `docs/PAYMENT_CN.md`、`docs/PAYMENT.md`。

### 8.2 兑换码销售价和返利

- 兑换码支持独立销售价，销售价可与面额区分。
- 返利来源不仅是支付订单，也可以是兑换码；账本保存 `source_type`、来源引用、兑换码类型/分组/倍率等快照。
- 返利基数按模型广场充值汇率换算，避免人民币销售价直接按美元计算。
- 支持冻结期、有效期、单被邀请人上限、管理员充值是否返利。
- 返利记录和转账记录可审计，重复来源通过幂等约束避免重复发放。

> 该部分作为 HCAI 定制的支付/返利链路，涉及前端、后端和数据库迁移。其主要作用为兑换码方式下的返利计算优化

数据迁移：`154_add_redeem_code_sale_price.sql`、`131_affiliate_rebate_hardening.sql`、`132_affiliate_custom_settings.sql`、`133_affiliate_rebate_freeze.sql`、`134_affiliate_ledger_audit_snapshots.sql`、`155_affiliate_ledger_source_ref.sql`。

## 9. 企业微信告警与运维模块

### 9.1 企业微信

- 告警规则和告警事件可通过企业微信群机器人发送。
- 管理端提供配置读取、保存和测试接口：`/admin/ops/wecom-notification/config`、`test`。
- webhook URL 在审计日志中按敏感字段脱敏。
- 去重键清理只按 HCAI 前缀删除，避免误删运行配置。

实现：`ops_wecom_notification.go`、`ops_alert_evaluator_service.go`、`ops_alerts_handler.go`、`ops_settings_handler.go`；迁移：`192_ops_wecom_alert_notification.sql`。

### 9.2 多节点运维

- 节点有稳定 `node_id` 和运维 `region`（例如 japan/taiwan/us）。
- Dashboard、内容审核、系统指标、错误日志和吞吐可按节点聚合。
- 告警评估、定时任务、备份和频道监控通过 leader lock/租约避免多实例重复执行。
- OAuth 刷新使用租约；缓存失效使用 outbox + Redis 广播，避免多节点读到旧权限。
- CN provider 余额/配额检查可门控到指定节点；Ollama Cloud usage 支持跨节点探测。

关键配置和实现：`config.go` 的 `global_background_tasks.disabled`、`ops.node_id`、`ops.region`、`api_key_auth_cache.invalidation_scope`；`leader_lock.go`、`oauth_refresh_lease.go`、`auth_cache_invalidation_outbox_repo.go`、`channel_monitor_runner.go`、`dashboard_aggregation_service.go`。运维默认值说明见 `docs/channel-monitor-v2-safe-defaults.md`。

## 10. 风控与内容审核

- 风控中心有独立入口和运行时开关 `risk_control_enabled`。
- 内容审核配置、审核端点和密钥管理在后台提供；敏感 Token 不进入普通日志。
- 内容审核节点上报健康状态、容量/延迟等指标，并形成运行时快照。
- 多节点状态在管理端聚合，可区分节点本地状态和集群状态。

入口：`backend/internal/service/content_moderation.go`、`content_moderation_repo.go`、`admin/content_moderation_handler.go`、`frontend/src/api/admin/riskControl.ts`、`RiskControlView.vue`。迁移：`135_content_moderation.sql`、`156_content_moderation_matched_keyword.sql`、`221_content_moderation_node_metrics.sql`。

## 11. 异步图片、媒体和用量记录

- 图片请求可创建异步任务，任务有状态、事件、错误和完成结果。
- 图片用量、实际尺寸、失败原因写入用量/错误记录，便于计费和审计。
- 支持 OpenAI Images、Grok media、批量图片任务和相关 failover。
- 任务处理支持集群协调，避免同一任务被多个节点重复消费。

入口：`async_image_ops.go`、`image_task_handler.go`、`grok_media.go`、`openai_images.go`、`image_task.go`；文档：`docs/ASYNC_IMAGE_TASKS.md`、`docs/BATCH_IMAGE_MVP.md`。

## 12. 网关与客户端兼容修复

与上游 `v0.2.7` 交叉对比后，明确需要记录的 HCAI 适配为：

- Anthropic Claude Code 使用的 base URL 去掉 `/v1`，避免拼接错误。
- Responses reasoning item ID、tool call ID 和 replayed ID 兼容。
- OpenAI Codex/CC Switch 默认模型版本维护。
- OpenAI 网关记录用量时识别异步图片来源，相关 handler 和 endpoint 分类同步适配。

其中 reasoning ID 是历史 HCAI 修复；当前相对上游的生产文件差异只剩注释，不能写成当前仍有一套独立 reasoning 算法。Chat Completions/Responses/Anthropic/Gemini 转换、Antigravity fallback、图片直连、WS 执行域等主要是上游能力，不列为 HCAI 原创。

这些改动分散在 `backend/internal/pkg/apicompat/`、`backend/internal/pkg/antigravity/`、`backend/internal/service/openai_*`、`gemini_*`、`grok_media.go` 和对应测试中；它们不是一个单独开关，合并上游时应以测试和 HCAI 提交记录为保护边界。

## 13. 更新、版本和发布体系

- 版本文件：`backend/cmd/server/VERSION`，当前值 `0.2.7-hcai`。
- 版本比较识别 `-hcai` 和 `+hcai` 后缀，不把同一数字版本的上游 release 误判成更新。
- 为避免误操作，移除了在线更新、回滚和重启接口；系统页只保留版本和检查更新能力。
- `.github/workflows/amd-linux-release.yml` 构建 Linux amd64：前端构建、`CGO_ENABLED=0` Go 构建、归档、checksum 和 `BUILD_INFO`。
- 当前 release 构建信息：`version=0.2.7-hcai`、`tag=v0.2.7-hcai`、`commit=4a1180444057`、`target=linux_amd64`。

### 部署风险

`deploy/install.sh`、Docker compose 和部分更新服务仍引用上游仓库 `Wei-Shaw/sub2api` 或镜像 `weishaw/sub2api:latest`。生产部署必须使用 HCAI release 包/镜像，不能执行默认上游安装脚本，否则可能重新安装上游版本并丢失行为和数据。

## 14. HCAI 合并保护清单

每次同步上游时至少检查：

1. `frontend/public/hcai/` 是否完整，`HomeView.vue` 是否仍有 loading/failed/ready 链路。
2. `plugin_management_enabled` 是否在 DTO、公开设置、更新服务和 Wire 装配中完整闭环。
3. 模型广场的四个设置键、人民币汇率、渠道价格优先级和中英文 locale 是否保留。
4. `open_mode`、外链 URL 清洗和 iframe 安全测试是否通过。
5. HCAI Switch OAuth callback、desktop fallback 和四种 usage mode 是否保留。
6. 支付回调、支付宝 sandbox、兑换码销售价和返利 source snapshot 迁移是否完整。
7. WeCom webhook 脱敏、去重键前缀清理和多节点 leader/租约是否保留。
8. 内容审核节点指标、异步图片用量/错误记录和网关兼容测试是否通过。
9. `VERSION`、amd64 release workflow、checksum/BUILD_INFO 和部署仓库地址是否指向 HCAI 产物。
10. 合并后运行前端 locale/static tests、模型广场测试、插件设置测试、Wire 生成测试及后端 CI。

## 15. 主要 HCAI 提交索引

| 提交 | 内容 |
| --- | --- |
| `7a0c2e022`、`8fa35fc52` | 兑换码销售价和返利账本 |
| `2e1e7521c`、`d3c656fb8` | 自定义菜单新窗口、分组菜单行为 |
| `3f31b6ffc`、`43fb72d8f`、`90dd38945` | HCAI 首页价格、模型和动态能力 |
| `5fcc9c3c7`、`31ccb2dbc`、`c56c7cbed` | CC Switch OAuth 桥接 |
| `b333c26a5` | amd64 release workflow |
| `a35015cbf` | Claude Code URL 兼容 |
| `94c9a02fd`、`943b36c88` | 移除在线更新/回滚/重启接口 |
| `c84cf6a89` | 异步图片用量和错误 |
| `09ee22219`、`df2e29909`、`785b61d42` | 模型广场汇率、过滤和图片价格 |
| `25d6edd04`、`a07a6f9e2` | 企业微信和外链安全修复 |
| `a1501cd44` | CC Switch 四种计费模式 |
| `c0b521b02`、`ba26f42d4` | 多区域任务、OAuth 租约和监控 |
| `658697f9f` | 支付宝沙箱 |
| `52cd37165`、`fbec96b03` | 风控和内容审核节点 |
| `450e0e3c5`、`13890907f`、`10daaeb2d` | 首页入口、主题、加载防闪烁 |
| `e633ed510`、`efb70b1b2` | 插件设置持久化和首页恢复 |
| `33bf9807a` | 模型广场价格与国际化恢复 |
| `90b4dfe24`、`fbd800329` | 渠道价格优先且限定为展示 |
| `06d6c8f92`、`b3ba58afc`、`84965a19d` | 上游版本同步并保留 HCAI |
| `9345eab94` | v0.2.7 合并后的后端依赖接线修复 |

## 16. 结论与边界

当前 `0.2.7-hcai` 的 HCAI 自定义核心是“品牌首页 + 模型广场商业展示 + 插件/菜单扩展 + CC Switch 接入 + 支付返利 + 多节点运维/风控”。协议兼容、图片任务和上游同步修复虽然部分实现来自上游，但在 HCAI 分支中已形成不可随意删除的行为约束。

本文档覆盖当前代码中可由提交、配置、迁移、入口或测试确认的 HCAI 定制。没有明确 HCAI 提交归属的纯上游功能未被强行归类；后续若新增 HCAI 功能，应同时补充本清单、对应测试和合并保护项。

## 17. 全量对比补充模块

### 17.1 上游账户商业信息、身份与额度

这是第一轮按 HCAI 名称搜索容易遗漏的定制，相关提交没有全部使用 hcai 前缀：`b9f8ffeab`、`9adc4089c`、`ee670ca57`、`e034b79ee`、`1c1000bff`。

- 上游倍率探测、批量探测、探测设置及账户单独开关；探测到期调度避免少数账户长期无法获得执行机会。
- 账户上游计费倍率历史，支持在账户列表中打开历史弹窗；前端有历史缓存。
- 上游服务商身份识别、站点 Logo 缓存及代理读取；包含 New API 新旧版图标资源。
- 账户/代理身份变更时使旧探测结果失效，持久化更新有 CAS 相关保护测试。
- 手动查询账户上游额度，返回余额、配额窗口、订阅等信息，区分认证失败、限流、超时、不支持协议及身份发生变化等错误。

前端：`frontend/src/components/account/UpstreamBillingRateCell.vue`、`UpstreamBillingRateHistoryDialog.vue`、`upstreamBillingRateHistoryCache.ts`、`AccountUsageCell.vue`、`frontend/src/views/admin/AccountsView.vue`。

后端：`backend/internal/service/upstream_billing_probe.go`、`upstream_billing_rate_history.go`、`upstream_identity_detection.go`、`upstream_site_logo.go`、`upstream_quota_query.go`，对应 handler、repository、测试全部列于附录。

API 在 `/api/v1/admin/accounts` 下：`/upstream-billing-rates`、`/upstream-billing-probe/settings`、`/upstream-billing-probe/batch`、`/upstream-site-logos/:key`、`/:id/upstream-billing-rate-history`、`/:id/upstream-billing-probe`、`/:id/upstream-quota/query`。这些是管理端接口，不应开放为匿名公共接口。

### 17.2 图片对象存储 MinIO 兼容

独立于备份 S3 基础功能，HCAI 在图片存储增加 `use_minio_client`：后台备份/图片存储页面可选择 minio-go 兼容客户端。配置经过前端 API、`ImageStorageSettings`、`config.ImageStorageConfig` 传递给 `image_storage_s3.go`。默认关闭；复用备份 S3 凭证时清理独立客户端设置。涉及 `backend/go.mod`、`go.sum` 依赖变化和上传测试，不能只保留前端复选框。

> 本项作为 HCAI 定制的图片存储兼容性改造。

### 17.3 API Key、账户操作与公共组件

- API Key 创建/编辑的分组选择器增加按平台过滤；`Select.vue` 增加 `filterOption` 和 `after-search` 插槽。
- 账户列表操作阻止事件冒泡，并局部更新账户状态，减少整页重载。
- 监控账户选择支持远程查询及加载态；当前 `Select.vue` 自身不包含 300ms 防抖计时器，不能仅凭历史提交标题认定当前组件仍有防抖。
- `HelpTooltip.vue` 增加打开延迟、键盘聚焦、上下自动选位、视口边界及高度约束，支持自定义 tooltip 样式。
- `AppLayout.vue` 改为固定视口布局，内容区域独立滚动并保持 scrollbar gutter 稳定。
- `formatScaled` 增加币种符号参数，默认美元，供人民币广场复用。
- CC Switch usage 脚本先转换非 ASCII 字符再 base64 编码，避免中文导致 `btoa` 抛错；四模式脚本的实际组装入口位于 `KeysView.vue`，`ccswitchImport.ts` 负责配置和 deeplink 编码。

### 17.4 运维与审核细项

- 邮件和企业微信均扩展账户异常、代理到期通知开关；WeCom 支持最低严重程度、每小时限额、恢复通知、webhook 配置状态/掩码和清除配置。
- 节点身份、节点指标、系统指标来源节点、节点上限与 dashboard 聚合有独立模型、仓储和测试。
- 备份启动时仅持有 leader lock 的实例修复遗留 running 记录，避免滚动升级误判其他节点正在执行的备份。
- 并发 slot 清理、定时账户测试、运维清理、聚合、告警和定时报表均是多节点协调保护范围。
- 提示词审计 `ConfigManager` 将公开配置快照与运行中 active snapshot 分开；配置加载失败时不能把可展示配置与实际生效配置混为一谈。
- 审计 middleware、服务层和日志脱敏器覆盖企业微信 webhook 等敏感数据。

### 17.5 异步图片的定制边界

异步/批量图片任务框架本身是上游基础。HCAI 的明确差异主要以记录链路、错误链路及 `RequestTypeAsync=6`为主：后端枚举/解析/兼容字段、数据库约束、网关用量记录、用户和管理员用量筛选、表格和错误徽标共同支持 `async`。本文档不把完整图片生成、视频或批量调度框架归为 HCAI 原创。

### 17.6 静态首页内容与外部依赖

除代码行为外，以下内容也属于品牌维护范围：客服公告、移动菜单、打字标题、模型卡切换、案例切换、FAQ、滚动锚点偏移；Sol/Terra/Luna 模型卡、图像案例图片和静态价格表等内容。

首页源码包含 `ai.hctopup.com` 接入示例、`chat.hctopup.com` 聊天入口、`ai-image.hctopup.com` 图片入口、企业微信客服、飞书文档及 `cdn.openai.com` 游戏 iframe。它们是配置/宣传内容及外部依赖，不代表本仓库同时实现了外部聊天或图片站点。聊天入口当前为 HTTP；静态价格表不会因渠道价格变更自动保证同步。CSP 的 `frame-src` 专门增加了 `https://cdn.openai.com`。本文仅静态核查，没有访问这些业务站点。

> 针对该部分内容 需在后续阶段进行优化

### 17.7 国际化与工程治理

- 中英文定制覆盖账户、渠道、运维、概览、资源、设置、通用、dashboard、misc；新增 `staticLocaleKeys.spec.ts` 做静态 key 检查，另有模块测试。HCAI 静态首页仍有中文硬编码，不能宣称整个品牌站完全双语。
- Node.js 22、pnpm 11.10.0、workspace overrides、锁文件和 eslint 忽略项是工程差异。
- 新统一 CI 针对 dev/main PR 运行 Go unit/full tests/build，以及前端 lint/typecheck/tests/build。
- 相对上游删除了独立 backend-ci、CLA、release、security-scan workflow；增加统一 CI、amd64 release、CODEOWNERS、PR 模板、分支保护说明和贡献说明。删除独立 security-scan 不等于已在统一 CI 中保留同等扫描。
- Wire 的源装配与生成结果需要一起验证。插件运行时和 PluginKVStore 本身来自上游；HCAI 是开关持久化修复与合并后接线修复，不是整套插件平台原创。

## 18. 数据、配置和验证索引

### 18.1 相对上游新增的 SQL 迁移（全部）

| 迁移 | 保护内容 |
| --- | --- |
| `154_add_redeem_code_sale_price.sql` | 兑换码销售价 |
| `155_affiliate_ledger_source_ref.sql` | 返利来源引用与审计 |
| `182_account_upstream_billing_rate_history.sql` | 上游倍率历史 |
| `183_upstream_site_logo_cache.sql` | 站点 Logo 缓存 |
| `191_allow_async_usage_request_type.sql` | 异步用量类型 |
| `192_ops_wecom_alert_notification.sql` | 企业微信告警字段 |
| `194_ops_node_metrics.sql` | 节点指标 |
| `195_auth_cache_invalidation_broadcast.sql` | 跨节点认证缓存失效 |
| `196_ops_system_metrics_source_node.sql` | 系统指标来源节点 |
| `197_oauth_refresh_leases.sql` | OAuth 刷新租约 |
| `221_content_moderation_node_metrics.sql` | 内容审核节点指标 |

路径统一在 `backend/migrations/`。第 5、8、10 节列出的其余迁移主要是依赖的上游表结构，不代表 HCAI 新增。迁移存在同数字不同文件名，必须按完整文件名核对，不能只按编号判重。Ent redeemcode schema、mutation、create/update 和 migrate/schema 生成文件也在附录中。

### 18.2 关键默认值和业务约束

`model_plaza_cny_per_usd` 默认 6.8。Token 人民币展示通常是基础单价 × 有效倍率 × 充值汇率 × 1,000,000；按次价格不乘百万。图片独立倍率模式不能再机械叠加普通分组倍率。广场参考折扣使用固定参考汇率 6.8，与可配置充值汇率不是同一个参数。

兑换码销售价 > 0 时，美元返利基数为销售价 / 充值汇率；未设销售价时只有余额兑换码回退面额，其余返回零。该设置因此不只是一个无副作用的 UI 参数。

环境配置包括 `GLOBAL_BACKGROUND_TASKS_DISABLED`、`OPS_NODE_ID`、`OPS_REGION`、`API_KEY_AUTH_CACHE_INVALIDATION_SCOPE`；YAML 对应第 9 节。图片 MinIO 开关为 `image_storage.use_minio_client`。实际线上启用值、数据库设置、插件安装列表、DNS/反代和节点部署内容不在 Git 仓库内，本文不能证明线上与代码一致。

### 18.3 本次盘点方法与验收边界

以 `git diff v0.2.7 9345eab94` 为净差异，另以 `git log 9345eab94 --not v0.2.7` 补历史归属，避免只搜索 hcai 字符串而漏掉无前缀提交。净差异共 367 个路径，包含新增、修改、删除、生成代码与测试；不等于 367 项原创功能。

附录完整列出所有路径与分支独有非 merge 提交，便于逐文件追溯。本文是仓库静态清单，不是所有功能已在线验收的承诺；本轮仅文档改动，未运行生产请求、迁移或完整业务测试。合并上线前仍需按第 14 节进行回归。

详见 [全量差异和提交附录](HCAI全量差异与提交附录.md)。
