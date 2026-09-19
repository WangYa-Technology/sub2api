# HCAI

<img src="frontend/public/hcai/hcai-logo.svg" alt="HCAI" width="200" />

面向多模型接入、计费与多区域运营的 AI API 网关。由 [WangYa-Technology](https://github.com/WangYa-Technology) 维护，基于 Sub2API 持续同步上游，并保留 HCAI 的品牌、业务与运维定制。

[版本发布](https://github.com/WangYa-Technology/sub2api/releases) · [问题反馈](https://github.com/WangYa-Technology/sub2api/issues) · [自定义功能清单](docs/hcai-dev/HCAI自定义功能清单.md) · [日本語](README_JA.md)

## 版本与仓库

- 本仓库：[`WangYa-Technology/sub2api`](https://github.com/WangYa-Technology/sub2api)。
- 当前源码版本：`0.2.7-hcai`，以 [VERSION](backend/cmd/server/VERSION) 为准。
- `dev`：开发和上游同步；`main`：通过 PR 合并的发布分支。
- HCAI 发布版本使用 `-hcai` 后缀。上游同数字版本不是 HCAI 发行包，不能直接替换。

本仓库提供网关及管理控制台；首页中链接的独立聊天、图片体验站点不属于本仓库的完整实现。

## 功能

网关基础能力包括多平台账户和渠道、模型路由、API Key、用量记录、余额与订阅计费、支付、管理控制台和协议兼容。模型是否可用取决于实际账户、渠道配置和服务商授权，不代表开箱即有可用额度。

HCAI 在此基础上维护以下定制：

| 模块 | HCAI 内容 |
| --- | --- |
| 品牌首页 | 独立首页、模型与案例展示、主题切换、文档入口、Logo 加载态防闪烁 |
| 模型广场 | 人民币实收价、美元官方参考价、充值汇率、参考折扣、中英文文案及渠道模型价格优先展示 |
| 插件与菜单 | 插件管理开关持久化、自定义菜单新窗口模式、外链与嵌入凭证隔离 |
| 客户端接入 | CC Switch 桌面 OAuth 回调、四种计费模式用量导入、中文脚本编码、Claude Code 地址适配 |
| 支付与返利 | 支付宝沙箱、兑换码销售价、销售额换算返利及来源账本 |
| 账户运营 | 上游站点识别与 Logo、计费倍率探测和历史、手动额度查询 |
| 多节点运维 | 节点与区域指标、全局任务协调、OAuth 刷新租约、跨节点认证缓存失效 |
| 告警与风控 | 企业微信通知、账户异常与代理到期通知、内容审核节点状态及集群视图 |
| 图片与存储 | 异步图片用量和错误记录、异步类型筛选、MinIO 图片存储兼容客户端 |
| 工程与发布 | HCAI 版本标识、统一 PR 检查、Linux amd64 发布包及校验文件 |

完整代码入口、配置键、迁移和上游边界见 [HCAI 自定义清单](docs/hcai-dev/HCAI自定义功能清单.md)，逐文件差异见 [差异与提交附录](docs/hcai-dev/HCAI全量差异与提交附录.md)。这些文档是代码盘点，不是线上功能全部通过验收的承诺。

### 价格口径

- 模型广场的渠道模型价格优先级仅用于展示，不修改真实请求的计费优先级。
- 官方参考价来自服务端价格目录、内置兜底及模型策略，不会因修改渠道售价而直接变化。
- `model_plaza_cny_per_usd` 用于人民币展示，也参与兑换码销售价的返利基数换算，修改前需核对业务影响。
- 首页静态价格表与模型广场不是同一数据源，维护价格时需要分别核对。

## 部署

### 推荐：本仓库发行包

从 [HCAI Releases](https://github.com/WangYa-Technology/sub2api/releases) 选择带 `-hcai` 后缀的版本。当前发布工作流构建 Linux amd64，包含：

- `sub2api_<版本>_linux_amd64.tar.gz`：程序及部署参考文件。
- `sub2api-linux-amd64.gz`：压缩程序。
- `checksums.txt`：SHA-256 校验。
- `BUILD_INFO.txt`：版本、提交、构建时间和目标平台。

部署前准备 PostgreSQL、Redis、持久化目录及 HTTPS 入口；配置可参考 [config.example.yaml](deploy/config.example.yaml) 和 [.env.example](deploy/.env.example)。首次启动支持初始化向导，不要将未初始化的管理入口长期暴露到公网。

升级前备份数据库、配置和对象存储相关凭据；核对下载包的校验值、构建提交及目标架构，再按实际节点的进程管理方式部署。配置数据库迁移后，回退旧二进制不一定能回退数据结构。

> **不要直接照搬上游一键安装命令或默认镜像。** 当前部分部署脚本和 compose 仍引用 `Wei-Shaw/sub2api`、`weishaw/sub2api:latest`；发行包附带的安装脚本也需要检查。它们不保证安装 HCAI。Docker 部署应自行构建并指定经过验证的 HCAI 镜像；本 README 不承诺存在可直接拉取的官方 HCAI 镜像。

HCAI 不提供控制台在线更新、回滚或重启接口。版本检查信息也不等同于允许覆盖安装；更新应由部署流程管理。

### 多区域部署

每个共享数据库的实例设置唯一且稳定的 `OPS_NODE_ID`，用 `OPS_REGION` 标记区域。跨 Redis 部署需要核对 `API_KEY_AUTH_CACHE_INVALIDATION_SCOPE`，并按节点职责配置 `GLOBAL_BACKGROUND_TASKS_DISABLED`，避免全局任务重复执行或全部无人执行。

参考 [多区域验证环境](deploy/tests/multiregion/README.md) 和 [监控默认值说明](docs/channel-monitor-v2-safe-defaults.md)。示例不是生产拓扑，不包含实际服务器地址和凭据。

## 本地开发

### 环境

- Go：以 [backend/go.mod](backend/go.mod) 为准，当前为 `1.27.0`。
- Node.js `22`、pnpm `11.10.0`：与 HCAI CI 保持一致。
- 可用的 PostgreSQL 与 Redis，建议使用独立开发实例。

```bash
git clone https://github.com/WangYa-Technology/sub2api.git
cd sub2api
git switch dev
pnpm --dir frontend install --frozen-lockfile
```

后端在单独终端运行，按初始化向导或部署配置连接开发数据库：

```bash
cd backend
go run ./cmd/server
```

前端在仓库根目录启动：

```bash
pnpm --dir frontend dev
```

默认前端地址为 `http://localhost:3000`，开发代理指向 `http://localhost:8080`。可通过 `VITE_DEV_PORT` 和 `VITE_DEV_PROXY_TARGET` 调整；详见 [Vite 配置](frontend/vite.config.ts)。

### 构建内嵌前端的程序

先构建前端，再使用 `embed` 标签编译后端。以下为在构建机器上生成本机架构程序的示例：

```bash
pnpm --dir frontend run build
cd backend
HCAI_BUILD_VERSION="$(./scripts/resolve-version.sh)"
CGO_ENABLED=0 go build -tags embed -trimpath \
  -ldflags="-s -w -X main.Version=${HCAI_BUILD_VERSION}" \
  -o bin/sub2api ./cmd/server
```

前端输出位于 `backend/internal/web/dist`。正式 Linux amd64 发布以 [发布工作流](.github/workflows/amd-linux-release.yml) 为准，不要把开发用的无前端构建当作完整发行包。

### 检查

在仓库根目录执行前端检查：

```bash
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
```

在 `backend` 目录执行后端检查：

```bash
go test -tags=unit ./...
go test ./...
CGO_ENABLED=0 go build ./cmd/server
```

以上对应 [PR CI](.github/workflows/ci.yml) 的主要检查；带 integration 标签的测试及多区域验证需另行准备依赖环境。

## 目录与文档

| 路径 | 内容 |
| --- | --- |
| `backend/` | Go API、网关、服务、数据库迁移和测试 |
| `frontend/` | Vue 3 / TypeScript 控制台 |
| `frontend/public/hcai/` | HCAI 品牌首页及静态资源 |
| `deploy/` | 配置、systemd、容器和多区域部署参考 |
| `.github/workflows/` | CI 与 HCAI 发布工作流 |
| `docs/` | 业务说明、自定义清单及合并记录 |

- [支付配置](docs/PAYMENT_CN.md)
- [插件开发](docs/PLUGIN_DEVELOPMENT.md) / [插件 API 与安全模型](backend/pkg/pluginapi/README.md)
- [异步图片任务](docs/ASYNC_IMAGE_TASKS.md)
- [组合分组](docs/COMPOSITE_GROUPS.md)
- [v0.2.7 合并记录](docs/UPSTREAM_0.2.7_HCAI_MERGE_CN.md)

## 开发与上游同步

项目规范：

- [开发规范](docs/hcai-dev/开发规范.md)
- [上游更新合并规范](docs/hcai-dev/上游更新合并规范.md)
- [HCAI 提交消息规范](docs/hcai-dev/HCAI提交消息规范.md)
- [版本发布规范](docs/hcai-dev/版本发布规范.md)
- [部署与多区域运维规范](docs/hcai-dev/部署与多区域运维规范.md)

开发改动先进入 `dev`，经检查后向 `main` 发起 PR。提交消息沿用 `hcai(fix): ...`、`hcai(feat): ...`、`hcai(docs): ...` 等格式。

开发、合并、审查、发布和部署必须同步更新 `docs/hcai-dev/` 中受影响文档；无更新需求时也必须说明核查范围和理由。缺少必要文档更新不得视为交付完成或合并就绪，具体要求见 [开发规范第 5 节](docs/hcai-dev/开发规范.md)。

同步上游时必须记录冲突位置、取舍和验证结果，不能整体覆盖 HCAI 首页、定价展示、插件开关、菜单安全、多节点配置或中英文文案。更新版本文件前核对实际合入内容；纯 Markdown 更新不触发发行，其余 `main` 推送进入发布门控，已有版本标签不会重新发布。发布前需检查版本号和产物来源。

## 来源、许可证与使用责任

本项目基于 [Sub2API](https://github.com/Wei-Shaw/sub2api) 二次开发，感谢上游作者和贡献者。保留上游来源与许可证说明不代表使用上游 README、赞助推广或上游发行包作为本项目入口。

本仓库许可证见 [LICENSE](LICENSE)（GNU LGPL v3）。原有版权声明与第三方组件的许可义务仍须保留；本 README 不修改许可条款，也不授予任何服务商品牌或账户使用授权。

部署者需自行确认所接入服务商的条款、账户权限及当地法律要求，妥善管理 API Key、OAuth Token、支付凭据和用户数据。请勿把生产密钥、真实请求内容或用户隐私提交到仓库或公开 Issue。
