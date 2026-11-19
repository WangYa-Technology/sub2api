# GPT-6.2 指令缓存修复记录

日期：2026-11-19。基线：本地 `dev` / `origin/dev` 的 `fe49e19c58b6a5aa8bead0d966eb3cd2c914e5b0`，应用版本 `0.2.22-hcai`。本次是未提交、未发布的本地修复；不代表 0.2.23 已合并或 GPT-6.2 已对外开放。

## 原因与范围

[上游 issue #8930](https://github.com/Wei-Shaw/sub2api/issues/8930) 报告 GPT-6.2 的顶层 `instructions` 按未缓存输入计费。[OpenAI 文档](https://developers.openai.com/api/docs/guides/keep-prefix-unchanged)建议把可复用开发者指令放入 `input` 首个 `developer` 消息，顶层字段留给回合指令。旧转发在缺少该字段时注入长默认指令，且会把 `system` 文本提升到顶层。issue 的约 5.4k token 增量是上游用户在 v0.2.23 的测量，不是 HCAI 生产实测。

修改范围：`openai_codex_transform.go`、`openai_gateway_forward.go`、`openai_gateway_messages.go` 及对应测试。仅对 `gpt-6.2-sol` / `gpt-6.2-astra`（含上游定义的 effort、`max` 与 `openai-compact` 后缀）启用新逻辑；0.2.22 当前未内置 GPT-6.2 映射，待上游 0.2.23 合并后重新核对模型目录与专属默认指令。

## 本地实现行为

1. 普通 OAuth/SetupToken Responses 转发不再为 GPT-6.2 预先注入顶层长默认指令。转换阶段将 `system` 改为原位 `developer`；已有首条 `developer` 时不再重复补默认指令；否则把默认指令放在 `input` 开头。顶层缺失或类型无效时写空字符串以满足既有 Codex 内部接口字段形态。客户端明确传入的非空顶层指令保持原样。
2. Chat Completions 及 Messages 兼容入口共用上述转换。Messages 的 `forced_codex_instructions_template` 对 GPT-6.2 渲染后放入首条 `developer`，旧模型仍写顶层。模板如含每回合变化内容，前缀仍会变化，须由配置维护者保持稳定。
3. OAuth 透传在请求模型名不含 `codex` 时原本不注入默认指令；此路径未改。原生 API Key 路径不进入 OAuth 默认指令转换。GPT-6.1 及更早模型保持原有行为。`/responses/compact` 保持既有形态。

## 验证与待完成

已用 HTTP stub 核对 GPT-6.2 普通 OAuth 的出站 JSON、透传无注入、显式顶层指令、`system` 转换、Messages 强制模板，以及旧模型回归。验证基于当前本地未提交工作树：

| 命令 | 结果 |
| --- | --- |
| `go test ./internal/service -run 'Test(ApplyCodexOAuthTransform\|OpenAIGatewayService_OAuth\|OpenAIGatewayService_GPT62\|ForwardAsAnthropic\|OpenAIResponsesInstructionsEcho)' -count=1` | 通过 |
| `go test ./internal/service -run 'Test(IsGPT62CodexModel\|OpenAIGatewayService_GPT62CodexInstructionsPlacement\|OpenAIGatewayService_OAuthPassthrough_GPT62DoesNotInjectInstructions\|ForwardAsAnthropic_GPT62ForcedInstructionsAreCacheable)' -count=1`；另以 `-tags=unit` 定向复跑模型识别、压缩和普通转发 | 通过，含最后补充的模型后缀识别 |
| `go test -tags=unit ./...`、`go test ./...` | 全量通过；随后只补充 GPT-6.2 的 `max` / `openai-compact` 模型后缀识别，已用上方定向测试验证，未再次全量复跑 |
| `CGO_ENABLED=0 go build -o /tmp/sub2api-gpt62-review-server ./cmd/server` | 通过 |
| `pnpm --dir frontend install --frozen-lockfile`、`lint:check`、`typecheck`、`test:run`、`build` | 全部通过；前端测试 395 个文件、3209 项 |

**尚未验证**：真实 ChatGPT/Codex OAuth 上游是否接受空顶层字段与首条 `developer`、连续回合 `cached_tokens` 是否增加、实际输入计费和回答质量。未使用生产凭据或发付费请求。上述真实验收及 0.2.23 合并后的重测完成前，不据此放行 GPT-6.2 OAuth 普通转发；回退方式为恢复该修复提交前的指令转换行为，并暂不开放 GPT-6.2。

## 0.2.23 合并后补充

上述“未提交/待合并”是修复初始阶段的记录。修复已于 `2438ff3422d96a6a3c6fe6b87c1a8eae3fbfb59f` 提交并进入 `0.2.23-hcai(fix1)` 合并候选。上游现提供 `openai.GPT62ModelID` 与 Sol/Astra 模型目录，HCAI 识别函数已复用该接口，不再自行接受日期后缀。合并后的定向测试、全量验证及正式发行状态见[0.2.23 合并记录](上游合并记录-0.2.23-hcai(fix1).md)和[发布记录](版本发布记录-0.2.23-hcai(fix1).md)；真实 OAuth 缓存命中与计费仍未验证。
