# v0.2.7 上游同步与 HCAI 保留记录

## 合并基线

- HCAI 基线：`e009765e9042efac6b462bdcee5b8ae393341760`（`dev`，`0.2.4-hcai`）。
- 上游标签：`v0.2.7`，提交 `7484192016807acf55c6ef4f2827d371eb61ec1c`。
- 目标版本：`0.2.7-hcai`。

## 显式冲突与解决方案

本次 Git 合并报告 14 个冲突文件，处理原则是保留 HCAI 专属行为并合并上游新增能力。

| 位置 | 冲突内容 | 解决方案 |
| --- | --- | --- |
| `backend/cmd/server/VERSION` | HCAI `0.2.4-hcai` 与上游 `0.2.5` | 统一为 `0.2.7-hcai`。 |
| `backend/cmd/server/wire_gen.go` | HCAI 上游计费探测配置和 Ollama 多节点锁装配与上游生成顺序不同 | 保留 HCAI 的 `configConfig`、Leader 锁和 Ollama 服务依赖，复用单实例并保留限流服务注入。 |
| `backend/go.sum` | HCAI 与上游的 `cpuid` 校验和集合不同 | 取两侧依赖校验和并保持 Go 模块文件可验证。 |
| `backend/internal/handler/dto/settings.go` | 自定义菜单 `open_mode` 与上游 `hide_open_button` | 两个字段同时保留；iframe/外链模式和隐藏打开按钮均可配置。 |
| `backend/internal/handler/endpoint.go` | HCAI 异步图片端点与上游 Seedance 任务端点常量/规范化不同 | 保留 HCAI 异步图片端点、映射和上游端点推导，同时加入 `/api/v3/contents/generations/tasks`。 |
| `backend/internal/service/setting_update.go` | HCAI 模型广场汇率、插件开关与上游订阅开关写入逻辑 | 三者全部写入；保留模型广场汇率规范化和插件开关。 |
| `frontend/src/components/layout/AppHeader.vue` | HCAI 模型广场紧凑样式与上游无障碍属性/订阅开关 | 保留 HCAI 活跃态和响应式样式，加入 title、aria-label、订阅功能门控及站点计费模式标题。 |
| `frontend/src/components/layout/__tests__/AppSidebar.spec.ts` | HCAI 折叠菜单回归测试与上游订阅入口测试 | 两组测试并存，分别覆盖菜单渲染和订阅/购买入口门控。 |
| `frontend/src/i18n/locales/en/admin/overview.ts` | HCAI 创建/复制提示与上游批量删除文案 | 保留双方键值，保证英文完整。 |
| `frontend/src/i18n/locales/zh/admin/overview.ts` | 中文同上 | 保留双方键值，保证中英文键对齐。 |
| `frontend/src/views/admin/__tests__/ChannelMonitorView.grok.spec.ts` | HCAI 固定 9 个供应商断言与上游新增 OpenCode 后的 10 个供应商 | 更新为 10 个，并继续验证 Grok 默认端点、模型和布局。 |
| `frontend/src/views/user/KeysView.vue` | HCAI 分组平台筛选与上游 API Key 供应商分类/批量编辑 | 保留上游供应商分类和批量编辑；在创建分组下拉中恢复 HCAI 平台细分筛选，切换供应商时清空旧平台筛选。 |
| `frontend/src/views/user/__tests__/CustomPageView.spec.ts` | HCAI 跨域嵌入不携带用户凭据与上游隐藏打开按钮测试 | 两者同时保留；断言 URL 不含 `user_id`、token，并验证隐藏按钮和 iframe。 |
| `frontend/src/views/user/__tests__/KeysView.spec.ts` | HCAI 平台筛选测试与上游供应商选择测试 | 保留上游分类/重置测试并补回平台筛选回归，防止创建 API Key 时筛选功能退化。 |

## HCAI 自定义保留核查

- 首页、首页 loading 状态、HCAI 静态资源和模型广场组件未被上游覆盖。
- 模型广场渠道定价优先、官方价格展示、充值汇率、中英文国际化和插件管理开关链路继续保留。
- 多节点 Leader 锁、异步图片、对象存储、运维指标、渠道计费探测和跨域嵌入凭据隔离继续保留。
- 上游新增 OpenCode、Seedance、订阅批量操作、API Key 批量编辑及相关迁移已并入。

## 数据库与发布说明

上游新增迁移必须在发布窗口执行并先备份数据库。发布目标是推送 `dev`，创建 `dev` → `main` PR，并发布 `v0.2.7-hcai`；不直接合并 `main`。

## 验证记录

- PR：`https://github.com/WangYa-Technology/sub2api/pull/14`（`dev` → `main`）。
- Release：`https://github.com/WangYa-Technology/sub2api/releases/tag/v0.2.7-hcai`。
- 前端 Vitest：306 个测试文件、2293 个测试全部通过。
- 前端 lint、TypeScript 类型检查和生产构建全部通过。
- `git diff --check` 通过。
- Go 全量测试和编译在本机因 Go 工具进程长时间无输出、CPU 为 0 而终止；CI PR 将继续执行后端检查。
