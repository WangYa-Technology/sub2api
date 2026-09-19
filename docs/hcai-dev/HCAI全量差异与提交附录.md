# HCAI 全量差异和提交附录

基线：上游 `v0.2.7`（`7484192016807acf55c6ef4f2827d371eb61ec1c`）对比 HCAI `9345eab94`。本表不包含本轮新文档。

配合 [功能总文档](HCAI自定义功能清单.md) 阅读。A=新增，M=修改，D=删除。每个路径均保留，生成代码和测试不省略；路径不存在但标记 D 表示有意删除的上游文件。

## 文件净差异与独有提交

下方前半部为全部 367 个差异路径；后半部为全部分支独有非 merge 提交（含维护、测试和已被上游吸收的历史修复，不可全部视为原创功能）。

```text
A	.github/BRANCH_PROTECTION.md
A	.github/CODEOWNERS
A	.github/pull_request_template.md
A	.github/workflows/amd-linux-release.yml
D	.github/workflows/backend-ci.yml
A	.github/workflows/ci.yml
D	.github/workflows/cla.yml
D	.github/workflows/release.yml
D	.github/workflows/security-scan.yml
A	CONTRIBUTING.md
M	README_CN.md
M	backend/cmd/server/VERSION
M	backend/cmd/server/wire.go
M	backend/cmd/server/wire_gen.go
M	backend/cmd/server/wire_gen_test.go
M	backend/ent/migrate/schema.go
M	backend/ent/mutation.go
M	backend/ent/redeemcode.go
M	backend/ent/redeemcode/redeemcode.go
M	backend/ent/redeemcode/where.go
M	backend/ent/redeemcode_create.go
M	backend/ent/redeemcode_update.go
M	backend/ent/runtime/runtime.go
M	backend/ent/schema/redeem_code.go
M	backend/go.mod
M	backend/go.sum
M	backend/internal/config/config.go
M	backend/internal/config/config_test.go
M	backend/internal/handler/admin/account_upstream_billing_probe.go
M	backend/internal/handler/admin/account_upstream_billing_probe_test.go
A	backend/internal/handler/admin/account_upstream_billing_rate_history.go
A	backend/internal/handler/admin/account_upstream_billing_rate_history_test.go
M	backend/internal/handler/admin/account_upstream_billing_rates.go
A	backend/internal/handler/admin/account_upstream_billing_rates_test.go
A	backend/internal/handler/admin/account_upstream_site_logo.go
M	backend/internal/handler/admin/admin_helpers_test.go
M	backend/internal/handler/admin/content_moderation_handler.go
M	backend/internal/handler/admin/ops_alerts_handler.go
M	backend/internal/handler/admin/ops_settings_handler.go
M	backend/internal/handler/admin/redeem_handler.go
M	backend/internal/handler/admin/setting_handler.go
M	backend/internal/handler/admin/setting_handler_audit.go
M	backend/internal/handler/admin/setting_handler_update.go
M	backend/internal/handler/admin/system_handler.go
D	backend/internal/handler/admin/system_handler_test.go
A	backend/internal/handler/async_image_ops.go
A	backend/internal/handler/async_image_ops_test.go
M	backend/internal/handler/auth_email_oauth_test.go
M	backend/internal/handler/dto/mappers.go
M	backend/internal/handler/dto/settings.go
M	backend/internal/handler/dto/types.go
M	backend/internal/handler/endpoint.go
M	backend/internal/handler/endpoint_test.go
M	backend/internal/handler/gateway_handler.go
M	backend/internal/handler/gateway_handler_usage_test.go
M	backend/internal/handler/gateway_handler_warmup_intercept_unit_test.go
M	backend/internal/handler/gateway_helper_fastpath_test.go
M	backend/internal/handler/gateway_helper_hotpath_test.go
M	backend/internal/handler/grok_media.go
M	backend/internal/handler/image_task_handler.go
M	backend/internal/handler/image_task_handler_test.go
M	backend/internal/handler/model_plaza_handler.go
M	backend/internal/handler/openai_images.go
M	backend/internal/handler/ops_error_logger.go
M	backend/internal/handler/redeem_handler_test.go
M	backend/internal/handler/wire.go
M	backend/internal/payment/provider/alipay.go
M	backend/internal/payment/provider/alipay_test.go
M	backend/internal/repository/account_repo.go
M	backend/internal/repository/account_repo_ollama_cloud_usage_integration_test.go
M	backend/internal/repository/account_repo_ollama_cloud_usage_test.go
M	backend/internal/repository/account_repo_upstream_billing_probe_cas_test.go
M	backend/internal/repository/account_repo_upstream_billing_probe_due_test.go
M	backend/internal/repository/account_repo_upstream_billing_probe_update_test.go
A	backend/internal/repository/account_upstream_billing_rate_history.go
A	backend/internal/repository/account_upstream_billing_rate_history_test.go
A	backend/internal/repository/account_upstream_site_logo.go
A	backend/internal/repository/account_upstream_site_logo_test.go
M	backend/internal/repository/affiliate_repo.go
M	backend/internal/repository/affiliate_repo_integration_test.go
M	backend/internal/repository/affiliate_repo_test.go
M	backend/internal/repository/auth_cache_invalidation_outbox_repo.go
M	backend/internal/repository/auth_cache_invalidation_outbox_repo_test.go
M	backend/internal/repository/concurrency_cache.go
M	backend/internal/repository/concurrency_cache_integration_test.go
M	backend/internal/repository/content_moderation_repo.go
M	backend/internal/repository/content_moderation_repo_test.go
M	backend/internal/repository/github_release_service.go
M	backend/internal/repository/github_release_service_test.go
M	backend/internal/repository/http_upstream.go
M	backend/internal/repository/http_upstream_test.go
M	backend/internal/repository/image_storage_s3.go
A	backend/internal/repository/image_storage_s3_test.go
M	backend/internal/repository/migrations_schema_integration_test.go
M	backend/internal/repository/ops_repo.go
M	backend/internal/repository/ops_repo_alerts.go
M	backend/internal/repository/ops_repo_metrics.go
M	backend/internal/repository/ops_repo_metrics_test.go
A	backend/internal/repository/ops_repo_node_metrics_test.go
A	backend/internal/repository/ops_repo_system_metrics_test.go
M	backend/internal/repository/proxy_repo.go
M	backend/internal/repository/proxy_repo_upstream_billing_probe_test.go
M	backend/internal/repository/redeem_code_repo.go
M	backend/internal/repository/upstream_billing_probe_persistence_integration_test.go
M	backend/internal/securityaudit/prompt_config_store.go
M	backend/internal/securityaudit/prompt_config_test.go
M	backend/internal/server/api_contract_test.go
M	backend/internal/server/middleware/audit_log.go
M	backend/internal/server/middleware/audit_log_test.go
M	backend/internal/server/routes/admin.go
A	backend/internal/server/routes/admin_upstream_quota_test.go
M	backend/internal/service/admin_account.go
M	backend/internal/service/admin_service.go
M	backend/internal/service/admin_user.go
M	backend/internal/service/affiliate_service.go
M	backend/internal/service/audit_log.go
M	backend/internal/service/audit_log_test.go
M	backend/internal/service/auth_cache_invalidation_outbox.go
M	backend/internal/service/auth_cache_invalidation_outbox_test.go
M	backend/internal/service/backup_service.go
M	backend/internal/service/billing_context_schedule.go
M	backend/internal/service/channel_monitor_runner.go
M	backend/internal/service/channel_monitor_runner_test.go
M	backend/internal/service/cn_provider_balance_check_service.go
M	backend/internal/service/cn_provider_balance_check_service_test.go
M	backend/internal/service/concurrency_service.go
M	backend/internal/service/concurrency_service_test.go
M	backend/internal/service/concurrency_slot_cleanup_test.go
M	backend/internal/service/content_moderation.go
M	backend/internal/service/content_moderation_test.go
M	backend/internal/service/dashboard_aggregation_service.go
M	backend/internal/service/dashboard_aggregation_service_test.go
M	backend/internal/service/domain_constants.go
M	backend/internal/service/gateway_multiplatform_test.go
M	backend/internal/service/image_storage_settings.go
M	backend/internal/service/image_storage_settings_test.go
M	backend/internal/service/image_storage_test.go
M	backend/internal/service/image_task.go
M	backend/internal/service/image_task_test.go
M	backend/internal/service/leader_lock.go
M	backend/internal/service/leader_lock_test.go
M	backend/internal/service/model_plaza_service.go
M	backend/internal/service/model_plaza_service_test.go
M	backend/internal/service/model_pricing_resolver.go
M	backend/internal/service/model_pricing_resolver_test.go
M	backend/internal/service/notification_email_service.go
M	backend/internal/service/notification_email_service_test.go
M	backend/internal/service/oauth_refresh_api.go
M	backend/internal/service/oauth_refresh_api_test.go
A	backend/internal/service/oauth_refresh_lease.go
M	backend/internal/service/ollama_cloud_usage.go
M	backend/internal/service/openai_gateway_apikey_item_id_test.go
M	backend/internal/service/openai_gateway_record_usage_test.go
M	backend/internal/service/openai_gateway_usage.go
M	backend/internal/service/openai_responses_item_id.go
M	backend/internal/service/ops_account_availability.go
M	backend/internal/service/ops_advisory_lock.go
M	backend/internal/service/ops_aggregation_service.go
M	backend/internal/service/ops_alert_evaluator_service.go
M	backend/internal/service/ops_alert_evaluator_service_test.go
M	backend/internal/service/ops_alert_models.go
M	backend/internal/service/ops_cleanup_executor.go
A	backend/internal/service/ops_cleanup_executor_test.go
M	backend/internal/service/ops_cleanup_service.go
M	backend/internal/service/ops_dashboard.go
M	backend/internal/service/ops_dashboard_models.go
A	backend/internal/service/ops_dashboard_node_limits_test.go
M	backend/internal/service/ops_metrics_collector.go
A	backend/internal/service/ops_node_identity.go
A	backend/internal/service/ops_node_metrics_test.go
M	backend/internal/service/ops_port.go
M	backend/internal/service/ops_realtime_models.go
M	backend/internal/service/ops_repo_mock_test.go
M	backend/internal/service/ops_scheduled_report_service.go
M	backend/internal/service/ops_service.go
M	backend/internal/service/ops_settings.go
M	backend/internal/service/ops_settings_models.go
A	backend/internal/service/ops_settings_resource_notifications_test.go
A	backend/internal/service/ops_wecom_atomicity_test.go
A	backend/internal/service/ops_wecom_notification.go
A	backend/internal/service/ops_wecom_notification_test.go
M	backend/internal/service/payment_config_providers.go
M	backend/internal/service/payment_config_providers_test.go
M	backend/internal/service/payment_fulfillment_test.go
M	backend/internal/service/payment_order_lifecycle_test.go
M	backend/internal/service/redeem_admin_fulfillment_test.go
M	backend/internal/service/redeem_code.go
M	backend/internal/service/redeem_code_test.go
M	backend/internal/service/redeem_rate_limit_test.go
M	backend/internal/service/redeem_service.go
M	backend/internal/service/redeem_service_batch_update_test.go
M	backend/internal/service/redeem_service_redeem_test.go
M	backend/internal/service/scheduled_test_runner_service.go
A	backend/internal/service/setting_model_plaza_test.go
M	backend/internal/service/setting_parse.go
M	backend/internal/service/setting_public.go
M	backend/internal/service/setting_service_update_test.go
M	backend/internal/service/setting_update.go
M	backend/internal/service/settings_view.go
M	backend/internal/service/update_service.go
M	backend/internal/service/update_service_test.go
M	backend/internal/service/upstream_billing_probe.go
A	backend/internal/service/upstream_billing_rate_history.go
A	backend/internal/service/upstream_billing_rate_history_test.go
A	backend/internal/service/upstream_billing_rates_test.go
A	backend/internal/service/upstream_identity_detection.go
A	backend/internal/service/upstream_quota_query.go
A	backend/internal/service/upstream_quota_query_test.go
A	backend/internal/service/upstream_site_logo.go
M	backend/internal/service/usage_log.go
M	backend/internal/service/usage_log_test.go
M	backend/internal/service/wire.go
M	backend/internal/testutil/stubs.go
M	backend/internal/util/logredact/redact.go
M	backend/internal/util/logredact/redact_test.go
A	backend/migrations/154_add_redeem_code_sale_price.sql
A	backend/migrations/155_affiliate_ledger_source_ref.sql
A	backend/migrations/182_account_upstream_billing_rate_history.sql
A	backend/migrations/183_upstream_site_logo_cache.sql
A	backend/migrations/191_allow_async_usage_request_type.sql
A	backend/migrations/192_ops_wecom_alert_notification.sql
A	backend/migrations/194_ops_node_metrics.sql
A	backend/migrations/195_auth_cache_invalidation_broadcast.sql
A	backend/migrations/196_ops_system_metrics_source_node.sql
A	backend/migrations/197_oauth_refresh_leases.sql
A	backend/migrations/221_content_moderation_node_metrics.sql
M	backend/migrations/auth_identity_payment_migrations_regression_test.go
A	backend/migrations/oauth_refresh_leases_migration_test.go
M	deploy/.env.example
M	deploy/config.example.yaml
M	deploy/docker-compose.dev.yml
A	deploy/tests/multiregion/README.md
A	deploy/tests/multiregion/docker-compose.yml
A	deploy/tests/multiregion/nginx.conf
A	deploy/tests/multiregion/run.sh
M	docs/PAYMENT.md
M	docs/PAYMENT_CN.md
A	docs/UPSTREAM_0.1.179_HCAI_AUDIT_CN.md
A	docs/UPSTREAM_0.2.7_HCAI_MERGE_CN.md
M	docs/channel-monitor-v2-safe-defaults.md
M	frontend/.eslintignore
M	frontend/package.json
M	frontend/pnpm-lock.yaml
A	frontend/pnpm-workspace.yaml
A	frontend/public/hcai/assets/image-2-showcase-CRhrZOEM.png
A	frontend/public/hcai/hcai-logo.svg
A	frontend/public/hcai/logo.png
A	frontend/public/hcai/main.js
A	frontend/public/hcai/page.html
A	frontend/public/hcai/planets.js
A	frontend/public/hcai/style.css
A	frontend/public/hcai/three.core.min.js
A	frontend/public/hcai/three.module.min.js
M	frontend/src/api/__tests__/admin.accounts.upstreamBillingProbe.spec.ts
D	frontend/src/api/__tests__/admin.system.rollback.spec.ts
M	frontend/src/api/admin/accounts.ts
M	frontend/src/api/admin/affiliates.ts
M	frontend/src/api/admin/backup.ts
M	frontend/src/api/admin/ops.ts
M	frontend/src/api/admin/redeem.ts
M	frontend/src/api/admin/riskControl.ts
M	frontend/src/api/admin/settings.ts
M	frontend/src/api/admin/system.ts
M	frontend/src/api/modelPlaza.ts
A	frontend/src/assets/upstream-providers/new-api-legacy.png
A	frontend/src/assets/upstream-providers/new-api-modern.png
M	frontend/src/components/account/AccountUsageCell.vue
M	frontend/src/components/account/BulkEditAccountModal.vue
M	frontend/src/components/account/UpstreamBillingRateCell.vue
A	frontend/src/components/account/UpstreamBillingRateHistoryDialog.vue
M	frontend/src/components/account/__tests__/AccountUsageCell.spec.ts
M	frontend/src/components/account/__tests__/UpstreamBillingRateCell.spec.ts
A	frontend/src/components/account/__tests__/UpstreamBillingRateHistoryDialog.spec.ts
A	frontend/src/components/account/upstreamBillingRateHistoryCache.ts
M	frontend/src/components/admin/account/AccountBulkActionsBar.vue
M	frontend/src/components/admin/monitor/MonitorFormDialog.vue
M	frontend/src/components/admin/usage/UsageFilters.vue
M	frontend/src/components/admin/usage/UsageTable.vue
M	frontend/src/components/common/HelpTooltip.vue
M	frontend/src/components/common/Select.vue
M	frontend/src/components/common/VersionBadge.vue
M	frontend/src/components/common/__tests__/HelpTooltip.spec.ts
M	frontend/src/components/common/__tests__/Select.spec.ts
M	frontend/src/components/keys/UseKeyModal.vue
M	frontend/src/components/keys/__tests__/UseKeyModal.spec.ts
M	frontend/src/components/layout/AppHeader.vue
M	frontend/src/components/layout/AppLayout.vue
M	frontend/src/components/layout/AppSidebar.vue
A	frontend/src/components/layout/__tests__/AppHeaderModelPlaza.spec.ts
A	frontend/src/components/layout/__tests__/AppLayoutScrollContainer.spec.ts
M	frontend/src/components/layout/__tests__/AppSidebar.spec.ts
M	frontend/src/components/layout/__tests__/docUrlSanitization.spec.ts
M	frontend/src/components/layout/__tests__/siteLogoSanitization.spec.ts
M	frontend/src/components/modelPlaza/ModelPlazaContent.vue
M	frontend/src/components/modelPlaza/PlazaFilterBar.vue
M	frontend/src/components/modelPlaza/PlazaGroupSection.vue
M	frontend/src/components/modelPlaza/PlazaModelPricingTable.vue
M	frontend/src/components/modelPlaza/__tests__/PlazaGroupSection.spec.ts
M	frontend/src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts
M	frontend/src/components/payment/PaymentProviderDialog.vue
M	frontend/src/components/payment/__tests__/PaymentProviderDialog.spec.ts
M	frontend/src/components/user/profile/ProfileIdentityBindingsSection.vue
A	frontend/src/i18n/__tests__/staticLocaleKeys.spec.ts
M	frontend/src/i18n/locales/en/admin/accounts.ts
M	frontend/src/i18n/locales/en/admin/channels.ts
M	frontend/src/i18n/locales/en/admin/ops.ts
M	frontend/src/i18n/locales/en/admin/overview.ts
M	frontend/src/i18n/locales/en/admin/resources.ts
M	frontend/src/i18n/locales/en/admin/settings.ts
M	frontend/src/i18n/locales/en/common.ts
M	frontend/src/i18n/locales/en/dashboard.ts
M	frontend/src/i18n/locales/en/misc.ts
M	frontend/src/i18n/locales/zh/admin/accounts.ts
M	frontend/src/i18n/locales/zh/admin/channels.ts
M	frontend/src/i18n/locales/zh/admin/ops.ts
M	frontend/src/i18n/locales/zh/admin/overview.ts
M	frontend/src/i18n/locales/zh/admin/resources.ts
M	frontend/src/i18n/locales/zh/admin/settings.ts
M	frontend/src/i18n/locales/zh/common.ts
M	frontend/src/i18n/locales/zh/dashboard.ts
M	frontend/src/i18n/locales/zh/misc.ts
M	frontend/src/main.ts
M	frontend/src/types/index.ts
M	frontend/src/utils/__tests__/ccswitchImport.spec.ts
M	frontend/src/utils/__tests__/embedded-url.spec.ts
A	frontend/src/utils/__tests__/errorBadges.spec.ts
A	frontend/src/utils/__tests__/theme.spec.ts
A	frontend/src/utils/__tests__/usageRequestType.spec.ts
M	frontend/src/utils/ccswitchImport.ts
M	frontend/src/utils/embedded-url.ts
M	frontend/src/utils/errorBadges.ts
M	frontend/src/utils/pricing.ts
A	frontend/src/utils/theme.ts
M	frontend/src/utils/usageRequestType.ts
M	frontend/src/views/HomeView.vue
M	frontend/src/views/__tests__/HomeView.compact.spec.ts
A	frontend/src/views/__tests__/HomeViewDocLink.spec.ts
A	frontend/src/views/__tests__/HomeViewModelPlaza.spec.ts
M	frontend/src/views/admin/AccountsView.vue
M	frontend/src/views/admin/BackupView.vue
M	frontend/src/views/admin/RedeemView.vue
M	frontend/src/views/admin/RiskControlView.vue
M	frontend/src/views/admin/SettingsView.vue
M	frontend/src/views/admin/UsageView.vue
A	frontend/src/views/admin/__tests__/AccountsView.upstreamQuota.spec.ts
M	frontend/src/views/admin/__tests__/GroupsView.codexManifest.spec.ts
M	frontend/src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts
M	frontend/src/views/admin/__tests__/RiskControlView.spec.ts
M	frontend/src/views/admin/__tests__/SettingsView.spec.ts
M	frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue
M	frontend/src/views/admin/ops/components/OpsAlertEventsCard.vue
M	frontend/src/views/admin/ops/components/OpsAlertRulesCard.vue
M	frontend/src/views/admin/ops/components/OpsDashboardHeader.vue
M	frontend/src/views/admin/ops/components/OpsEmailNotificationCard.vue
M	frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue
M	frontend/src/views/admin/ops/components/OpsErrorLogTable.vue
M	frontend/src/views/admin/ops/components/OpsRuntimeSettingsCard.vue
M	frontend/src/views/admin/ops/components/OpsSettingsDialog.vue
M	frontend/src/views/admin/ops/components/__tests__/OpsErrorLogTable.spec.ts
M	frontend/src/views/admin/ops/types.ts
M	frontend/src/views/admin/settings/EmailTemplateEditor.vue
M	frontend/src/views/auth/OAuthCallbackView.vue
M	frontend/src/views/auth/__tests__/OAuthCallbackView.spec.ts
M	frontend/src/views/user/CustomPageView.vue
M	frontend/src/views/user/KeysView.vue
M	frontend/src/views/user/UsageView.vue
M	frontend/src/views/user/__tests__/CustomPageView.spec.ts
9345eab94 hcai(fix): repair backend dependency wiring after v0.2.7 merge
d6016cbaa docs(hcai): record v0.2.7 merge validation
e009765e9 test: fix repository test assignment
4e0ca430d test: fix upstream merge CI regressions
fbd800329 hcai(fix): limit channel pricing priority to plaza display
90b4dfe24 hcai(fix): prioritize channel model pricing
33bf9807a hcai(fix): restore model plaza pricing and locale coverage
10daaeb2d hcai(fix): prevent homepage flash during loading
a256f6e73 test: update admin settings contract fixture
efb70b1b2 fix: restore plugin setting persistence and hcai home
790474868 hcai(fix): persist plugin management setting (#6)
e633ed510 hcai(fix): persist plugin management setting
bae2faede hcai(release): set version 0.1.183-hcai
5eba5aa1e hcai(merge): sync upstream v0.1.183 and retain HCAI features (#5)
2dce129f5 hcai(ci): regenerate server wiring
a60f0b601 hcai(ci): fix upstream check failures
b1d588bc3 hcai(ci): fix pnpm 11 workspace overrides
a90686dd1 hcai(test): align home view assertions with upstream implementation
77cda5c0c hcai(merge): align upstream model plaza and home components
91d5fad7c hcai(fix): resolve v0.1.183 merge build issues
296f4335a hcai(ci): establish reliable PR checks and remote account search (#4)
935d1858d hcai(ci): build backend without missing frontend artifacts
04467ae85 hcai(test): make CI checks environment independent
faa5be431 hcai(fix): enable debounced remote monitor account search
d0f8ed7e8 hcai(chore): establish dev branch workflow and CI governance
3e5ab4779 hcai(fix): pass config to channel monitor quota fetcher
e041c600b hcai(docs): add v0.1.179 deployment audit
5a141e290 hcai(fix): gate CN provider checks across nodes
13890907f hcai(feat): 实现首页主题切换功能，添加主题切换按钮和相关样式，支持根据系统设置和用户选择保存主题偏好
1daf877e3 hcai(fix): 更新 Node.js 版本至 22
f1d1c7d92 hcai(version): set version to 0.1.176-hcai(fix)
4b04bfae3 Merge pull request #5668 from Wei-Shaw/fix/codex-turn-state-and-fingerprint-optin
153210494 Merge pull request #5641 from InCerryGit/fix/issue-5624-remote-compaction-v2
450e0e3c5 hcai(fix): 更新 HomeView 以支持公共文档链接和模型广场入口，添加相关单元测试
fbec96b03 hcai(feat): 增加内容审核节点状态管理功能，支持集群聚合和运行时快照
52cd37165 hcai(feat): enhance risk control view with additional moderation endpoints and key management features
658697f9f feat(hcai): 添加支付宝沙箱环境支持及相关配置更新
a07a6f9e2 fix(hcai): 修复一系列问题 - 修复兑换码返利基数计算错误，人民币销售价应按模型广场充值倍率换算为美元 - 修复企业微信通知去重键清理逻辑，按前缀定向删除，避免误删运行配置 - 修复自定义菜单外部链接构建逻辑，避免泄露用户凭证和来源上下文
2e07dad22 feat(frontend): filter API key groups by platform
ba26f42d4 hcai(multiregion):完善 OAuth 刷新租约、后台任务协调及多节点监控
c0b521b02 hcai(fix): harden multi-region task coordination and observability
5d4a2f52e hcai(fix): update OpenAI CC Switch model version to gpt-5.6-sol
a1501cd44 hcai(fix): Optimize the CC-Switch import usage query script to support four billing modes (Wei-Shaw#5074) Refactor the hardcoded `usageScript` within `executeCcsImport`: - Error handling: Detect `error` or `isValid === false` and return `invalidMessage`. - Subscription mode: Pass through `used`/`total`; display progress percentages for daily, weekly, and monthly windows in the `extra` field. - Quota-limited mode: Include `used`, `total`, and percentage. - Rate-limited mode: Display the window with the highest usage rate as the primary metric; list all windows in the `extra` field. - Balance mode: Include `used` (derived from `usage.total.cost`) and `total`. - Pass through `planName` to distinguish between the four modes. - Unlimited subscription: Set `remaining` to `null` and `extra` to "Unlimited" to avoid displaying negative values. - Improve compatibility by using `var`/`function` syntax and specifying `User-Agent` and `Content-Type` headers.
4bdde94a5 hcai(fix): update click handlers to stop event propagation and patch account data without reloading
df2e29909 hcai(refactor): remove rate filtering from PlazaFilterBar and ModelPlazaContent
6d0aeb489 hcai(version): update version format by removing 'v' prefix
25d6edd04 feat(hcai): add WeCom notification support for alert rules and events
09ee22219 hcai-feat(model-plaza): implement CNY to USD conversion for pricing and enhance UI
6dcf5e969 hcai-fix(service): update comments and logic for reasoning item ID handling
c84cf6a89 feat(hcai): record async image usage and errors
943b36c88 refactor(hcai): remove update and restart endpoints, streamline update logic, and clean up frontend components
94c9a02fd feat(hcai):Refactor update service tests and remove rollback functionality
f5ce99319 chore(hcai): update pnpm version to 11.10.0
1c1000bff fix: harden upstream identity invalidation and logo handling
e034b79ee feat: detect upstream identity and site logo
708d7779b test: fix billing history integration fixtures
ee670ca57 feat: add upstream billing rate history
9adc4089c fix: prevent upstream probe starvation
b9f8ffeab feat(admin): enrich upstream billing information
a35015cbf ui(hcai): strip /v1 from Anthropic Claude Code base URL in UseKeyModal
b333c26a5 ci(hcai): Add GitHub Actions workflow for building and releasing Linux AMD64 deployment package
c56c7cbed hcai: decode desktop OAuth redirect
31ccb2dbc hcai: add desktop OAuth callback fallback
ec6f66431 chore: sync VERSION to 0.1.156
5fcc9c3c7 hcai: support CC Switch OAuth callback bridge
c32c16c6c chore: sync VERSION to 0.1.155
90dd38945 HCAI: 更新首页模型展示与动态能力模块
b8b1d5049 chore: sync VERSION to 0.1.153
43fb72d8f HCAI: 更新首页模型展示与动态能力模块
f3470c59d chore: sync VERSION to 0.1.150
c5d6c63c3 chore: sync VERSION to 0.1.146
cfb17cb67 chore: sync VERSION to 0.1.145
63e1d1839 chore: sync VERSION to 0.1.144
ed11d1b37 chore: sync VERSION to 0.1.143
3f31b6ffc Update HCAI home pricing table
d3c656fb8 hcai: prevent grouped sidebar items from rendering as standalone links
08fdb9917 hcai: bump displayed version to 0.1.139
2e1e7521c hcai: add an option to open sidebar menu items in a new window.
8fa35fc52 hcai: add records of turnover-based rebates for redemption codes
7a0c2e022 hcai: add redeem code sale price rebate support and hcai frontend
```
