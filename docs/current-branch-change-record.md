# Current Branch Change Record

Date: 2026-06-08

## Purpose

This document records the custom logic carried by `custom/main` so upstream
merges can be checked against it. During merges, preserve these behaviors unless
there is an explicit replacement with equivalent functionality.

## Branch And Baseline

- Custom branch: `custom/main`
- Upstream remote: `upstream` (`https://github.com/QuantumNous/new-api.git`)
- Custom baseline before the May 2026 upstream merge: `9f8a4ec05010da20704c1b55aa8b9af5630df72e`
- Last audited custom head before the June 2026 upstream merge:
  `ab2e531ad08b3192e6dfeb2890b09c534cc5c87b`
- Last audited upstream head for that merge:
  `4ca47ee236fd5f09ec71d732fbe3d4270f03bec3`
- Local custom commits on top of the May 2026 baseline:
  - `f9a15ba0d` feat: add usage analytics and relay tracing
  - `7f9784556` fix channel modal model selector and batch settings
  - `ae603ecd1` chore: configure custom docker deployment
  - `0e9d5b1ce` fix docker frontend build plugin tracking
  - `f5d4eb2c3` feat: improve channel batch creation workflow
  - `44bdd211c` feat: enhance database configuration and error handling in environment initialization
  - `3dc4cab94` feat: implement channel failure rate monitoring and webhook notifications
  - `09dd1291e` feat: enhance ImageRequest structure and update reasoning content handling in messages
  - `ae53cdc9e` feat: add vendor profile management and enhance channel filtering options
  - `bd31a7ad5` feat: implement usage aggregation tasks and enhance related configurations
  - `e455899a2` feat: implement channel archiving functionality and enhance related filtering options
  - `87049b8f7` refactor: normalize vendor profile ID handling in channel model
  - `c84856223` feat: implement log retention and trace storage features
  - `85c93b774` feat: enhance trace logging and storage capabilities
  - `ef1987907` feat: add tests for webhook options and enhance webhook handling
  - `fc08f73dc` feat: enhance channel error handling and webhook notifications
  - `51ee4d458` feat: enhance channel management with auto-disable policy group support
  - `8cd4da5ce` refactor: update log model and related functions for improved handling of log IDs
  - `a62b43f68` feat: add batch upload configuration for trace logging
  - `152eb2270` feat: enhance channel settings with auto recovery feature
  - `9d61c2d3a` feat: improve error handling and trace payload management
  - `d62369f14` feat: enhance retry logic with detailed reasons for failure
  - `79b22bf7c` feat: enhance Docker Compose configuration and relay error handling
  - `47563acdb` feat: enhance dashboard quota data retrieval and testing
  - `ab2e531ad` feat: enhance usage aggregation and provider key retrieval

## Frontend Merge Policy

- Upstream's new frontend may be merged in full.
- The classic/old frontend is the default frontend for this branch.
- Existing custom frontend features must be preserved in `web/classic` after
  upstream renames the old frontend from `web/src` to `web/classic/src`.
- Do not drop custom pages, sidebar entries, table columns, filters, i18n keys,
  or API helpers just because upstream moved the frontend layout.
- New frontend parity is optional unless explicitly requested. If custom features
  are later ported to `web/default`, keep `web/classic` behavior as the source
  of truth until the port is verified.
- Some custom features have already been ported to `web/default`. Do not drop
  those ported views, filters, columns, APIs, or i18n keys during upstream
  merges just because `web/classic` remains the default frontend.

## Custom Features To Preserve

### Usage Analytics And Relay Tracing

- Provider key observability:
  - `ProviderKey` model keyed by SHA-256 fingerprint.
  - Admin API for provider key listing.
  - Log records store `provider_key_id`, upstream request metadata, upstream key
    preview/current key, and provider-key filters.
- Cost-aware usage accounting:
  - `Log.cost_quota`.
  - Channel `cost_ratio` setting.
  - Dashboard and usage queries can switch between original quota and cost quota.
- Dashboard dimensions:
  - Model, upstream key/provider key, channel, token, username, vendor/profile,
    and other custom dimensions where implemented.
  - Admin dashboard filters must keep provider key, model, channel, token,
    metric, and time range support.
  - Dashboard quota queries can read from `UsageAggregate` tables and live
    `UsageLedger` rows, and must keep provider-key, vendor-profile, group,
    user, and business-dimension filters.
- Relay trace capture:
  - Request headers, response headers, upstream request ID, upstream status, and
    bounded request/response body previews.
  - Binary and multipart bodies are represented by metadata instead of inline
    content.
  - Trace storage can upload S3-compatible objects, batch trace archives, and
    optional full request/response bodies controlled by `TRACE_*` settings.
  - Log trace retention/cleanup must preserve metadata consistency and must not
    orphan temporary disk cache files.
  - Stream scanner idle timeout handling and trace capture must remain intact.
  - Trace payload transfer is single-use; do not duplicate capture calls or
    lose captured payloads before log generation.
- Log storage and query behavior:
  - `Log.Id` is generated as an int64-compatible ID and log queries must remain
    compatible with sharded log tables.
  - Log count/list/stat queries must continue to work across shards and through
    read-replica fallback paths.
  - Obsolete shard indexes should stay removed; do not reintroduce migrations
    that fail the log-index compatibility tests.
- Classic frontend:
  - Credential/provider-key page.
  - Dashboard filter and analysis table additions.
  - Usage-log trace viewer, provider-key filter, request-ID prefill, and expanded
    upstream metadata display.
- Default frontend:
  - Usage summary page/API integration, usage-log trace details, provider-key
    and vendor-profile display, and monitoring settings parity must be kept.

### Channel Creation And Management

- Channel batch creation workflow:
  - Server-side batch channel handling and validation tests.
  - Classic channel modal batch inputs and model selector fixes.
  - Batch-created rows must retain expected status, group, model, key, and
    channel settings behavior.
- Channel archive behavior:
  - Archived channels are excluded from runtime distribution and normal channel
    selection.
  - Archive/unarchive APIs and classic UI actions are preserved.
  - Channel tags, filtering, testing, billing, upstream update, MJ/task relay,
    and Codex credential refresh logic must respect archived channels.
  - Channel cache and ability queries must not accidentally include archived
    channels where active-only behavior is expected.
- Vendor profile management:
  - `VendorProfile` model and API.
  - Channel association with vendor profiles.
  - Dashboard, usage logs, and channel filters can filter/group by vendor profile.
  - Classic vendor profile modal and channel table/filter integration are kept.
  - Vendor profile ID `0` is normalized to `NULL` in channel writes and can be
    cleared without storing an invalid zero foreign key.
- Channel disable/recovery policies:
  - Per-channel `auto_disable_policy_group` is stored in channel settings and
    propagated into relay errors, channel tests, billing disables, usage
    aggregates, provider-key listings, and frontend channel forms.
  - Per-channel auto recovery mode can override the global automatic recovery
    setting; cache and selection logic must respect enabled/disabled/default.
  - Retry skip reasons, request-context-canceled handling, and channel-affinity
    skip-retry behavior must remain intact.

### Monitoring And Operations

- Channel failure-rate monitoring:
  - `service/channel_health.go` failure-rate monitor.
  - Webhook notification payload/configuration changes.
  - Webhook secrets are stored/loaded but must not be leaked through option
    responses.
  - Operation settings for failure-rate thresholds and notification behavior.
  - Classic monitoring settings UI.
  - Default frontend monitoring settings UI where already ported.
- Usage aggregation:
  - Scheduled usage aggregation task registration in `main.go`.
  - `UsageAggregate` model/API/page.
  - Usage ledger integration remains compatible with aggregation.
  - Hourly/daily bucket recompute, retention, export limits, timezone handling,
    and provider-key retrieval must stay covered.
  - Sidebar module and notification settings entries for usage aggregation remain
    available in the classic frontend.
- Database and runtime configuration:
  - Enhanced DB initialization error handling.
  - Docker compose customizations, `docker-compose.polo.yml`, and
    `.env.production.example`.
  - RDS/read-replica DSNs, separate log DB/read DSNs, Redis credentials, trace
    storage, log retention, and relay idle connection timeout env settings.
  - Tag/channel migration compatibility across SQLite, MySQL, and PostgreSQL.
  - Task polling and tag handling regressions covered by tests must stay fixed.
- Build and deployment:
  - Custom Docker build workflow and pinned Bun/Go builder stages must keep
    building both `web/default` and `web/classic` assets.
  - Classic frontend build compatibility helpers should not be removed unless
    the replacement build path is verified.
  - Trace storage/retention deployment notes in
    `docs/log-trace-s3-retention-design.md` should stay aligned with env vars.

### Relay And Request DTO Semantics

- Optional request fields parsed from client JSON and re-marshaled upstream must
  preserve explicit zero/false values using pointer fields where needed.
- Message reasoning fields must preserve explicitly empty reasoning content.
- OpenAI image request/edit fields must preserve reference fields expected by
  upstream providers.
- Vertex URL building must support custom base URL / gateway prefix behavior.
- Relay HTTP client cache keys must include proxy and timeout policy where custom
  timeout behavior exists.
- Relay HTTP transport must keep `RELAY_IDLE_CONN_TIMEOUT` support.
- Converted JSON request bodies should use the shared outbound-body helper so
  body size and trace capture stay correct.

## Key Backend Files

- Controllers:
  - `controller/channel.go`
  - `controller/channel-billing.go`
  - `controller/channel-test.go`
  - `controller/channel_upstream_update.go`
  - `controller/log.go`
  - `controller/option.go`
  - `controller/provider_key.go`
  - `controller/ratio_sync.go`
  - `controller/relay.go`
  - `controller/relay_retry_test.go`
  - `controller/usage_aggregate.go`
  - `controller/usedata.go`
  - `controller/vendor_profile.go`
- Models:
  - `model/channel.go`
  - `model/channel_cache.go`
  - `model/dashboard_usage.go`
  - `model/log.go`
  - `model/log_id.go`
  - `model/log_shard.go`
  - `model/log_trace.go`
  - `model/main.go`
  - `model/option.go`
  - `model/provider_key.go`
  - `model/provider_key_query.go`
  - `model/usage_aggregate.go`
  - `model/usage_ledger.go`
  - `model/vendor_profile.go`
- Relay/service:
  - `dto/channel_settings.go`
  - `pkg/tracestore/tracestore.go`
  - `relay/common/trace_capture.go`
  - `relay/common/relay_info.go`
  - `relay/helper/stream_scanner.go`
  - `relay/channel/**`
  - `service/channel_health.go`
  - `service/channel_select.go`
  - `service/webhook.go`
  - `service/http_client.go`
  - `service/log_info_generate.go`
  - `service/log_retention_task.go`
  - `service/log_trace_capture.go`
  - `service/usage_aggregation_task.go`
  - `types/channel_error.go`
  - `types/error.go`
- Routing/config:
  - `router/api-router.go`
  - `main.go`
  - `Dockerfile`
  - `.github/workflows/custom-docker.yml`
  - `docker-compose.yml`
  - `docker-compose.polo.yml`
  - `.env.example`
  - `.env.production.example`
  - `docs/log-trace-s3-retention-design.md`
  - `setting/operation_setting/auto_disable_policy.go`
  - `setting/operation_setting/monitor_setting.go`
  - `common/init.go`
  - `common/constants.go`

## Key Classic Frontend Files

After upstream's v1 frontend merge, these custom old-frontend files should live
under `web/classic/src`:

- `web/classic/src/App.jsx`
- `web/classic/package.json`
- `web/classic/rsbuild.config.ts`
- `web/classic/build/vite-plugin-semi-compat.js`
- `web/classic/src/components/dashboard/ChartsPanel.jsx`
- `web/classic/src/components/dashboard/DashboardAnalysisTable.jsx`
- `web/classic/src/components/dashboard/DashboardFilters.jsx`
- `web/classic/src/components/dashboard/DashboardHeader.jsx`
- `web/classic/src/components/dashboard/index.jsx`
- `web/classic/src/components/layout/PageLayout.jsx`
- `web/classic/src/components/layout/SiderBar.jsx`
- `web/classic/src/components/settings/OperationSetting.jsx`
- `web/classic/src/components/settings/personal/cards/NotificationSettings.jsx`
- `web/classic/src/components/table/channels/ChannelsActions.jsx`
- `web/classic/src/components/table/channels/ChannelsColumnDefs.jsx`
- `web/classic/src/components/table/channels/ChannelsFilters.jsx`
- `web/classic/src/components/table/channels/ChannelsTabs.jsx`
- `web/classic/src/components/table/channels/index.jsx`
- `web/classic/src/components/table/channels/modals/EditChannelModal.jsx`
- `web/classic/src/components/table/channels/modals/VendorProfileModal.jsx`
- `web/classic/src/components/table/provider-keys/index.jsx`
- `web/classic/src/components/table/usage-logs/UsageLogsColumnDefs.jsx`
- `web/classic/src/components/table/usage-logs/UsageLogsFilters.jsx`
- `web/classic/src/components/table/usage-logs/index.jsx`
- `web/classic/src/components/table/usage-logs/modals/TraceModal.jsx`
- `web/classic/src/helpers/api.js`
- `web/classic/src/helpers/dashboard.jsx`
- `web/classic/src/helpers/render.jsx`
- `web/classic/src/hooks/channels/useChannelsData.jsx`
- `web/classic/src/hooks/dashboard/useDashboardCharts.jsx`
- `web/classic/src/hooks/dashboard/useDashboardData.js`
- `web/classic/src/hooks/dashboard/useDashboardStats.jsx`
- `web/classic/src/hooks/provider-keys/useProviderKeysData.jsx`
- `web/classic/src/hooks/usage-logs/useUsageLogsData.jsx`
- `web/classic/src/pages/Credential/index.jsx`
- `web/classic/src/pages/Setting/Operation/SettingsMonitoring.jsx`
- `web/classic/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx`
- `web/classic/src/pages/Setting/Ratio/UpstreamRatioSync.jsx`
- `web/classic/src/pages/UsageAggregate/index.jsx`
- `web/classic/src/i18n/locales/*.json`

## Key Default Frontend Files

These files contain ported custom behavior in the default frontend and should
also be checked during upstream merges:

- `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- `web/default/src/features/channels/lib/channel-form.ts`
- `web/default/src/features/channels/types.ts`
- `web/default/src/features/dashboard/api.ts`
- `web/default/src/features/dashboard/components/usage-summary/usage-summary.tsx`
- `web/default/src/features/dashboard/index.tsx`
- `web/default/src/features/dashboard/section-registry.tsx`
- `web/default/src/features/dashboard/types.ts`
- `web/default/src/features/models/components/models-columns.tsx`
- `web/default/src/features/system-settings/integrations/monitoring-settings-section.tsx`
- `web/default/src/features/system-settings/models/channel-selector-dialog.tsx`
- `web/default/src/features/system-settings/models/upstream-ratio-sync.tsx`
- `web/default/src/features/system-settings/operations/index.tsx`
- `web/default/src/features/system-settings/operations/section-registry.tsx`
- `web/default/src/features/system-settings/types.ts`
- `web/default/src/features/usage-logs/api.ts`
- `web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx`
- `web/default/src/features/usage-logs/components/common-logs-filter-bar.tsx`
- `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`
- `web/default/src/features/usage-logs/data/schema.ts`
- `web/default/src/features/usage-logs/lib/format.ts`
- `web/default/src/features/usage-logs/types.ts`
- `web/default/src/hooks/use-sidebar-config.ts`
- `web/default/src/hooks/use-sidebar-data.ts`
- `web/default/src/lib/auto-disable-policy-groups.ts`
- `web/default/src/lib/dayjs.ts`
- `web/default/src/lib/time.ts`
- `web/default/src/i18n/locales/*.json`

## Merge Checklist

Use this checklist after every upstream merge:

1. Confirm old frontend defaults to classic and custom old-frontend files are
   under `web/classic`.
2. Confirm `/api/provider_key`, vendor profile APIs, usage aggregation APIs, and
   channel archive APIs are still routed.
3. Confirm channel list/distribution paths exclude archived channels by default.
4. Confirm provider key, cost quota, vendor profile, request ID, and trace fields
   are still populated in logs.
5. Confirm dashboard and usage-log filters still accept provider key, vendor
   profile, channel, model, token, metric, and date parameters.
6. Confirm failure-rate monitor settings still load from operation settings and
   still send webhook notifications.
7. Confirm auto-disable policy groups and per-channel auto recovery still affect
   channel disable/enable decisions, relay errors, and channel cache behavior.
8. Confirm log sharding, generated log IDs, read-replica fallback, and obsolete
   index cleanup migrations still pass model tests.
9. Confirm trace storage modes, full-body capture, batch uploads, and retention
   cleanup still work without duplicate request/response body capture.
10. Confirm relay retry reasons, request-context-canceled handling, channel
    affinity skip-retry, and HTTP client proxy/timeout/idle-timeout behavior.
11. Confirm optional relay DTO fields still preserve explicit zero/false values.
12. Confirm RDS/read-replica/log DB Docker/env settings are preserved.
13. Run backend tests that cover custom models, relay, channel filtering, and
   aggregation.
14. Run classic frontend build; run default frontend build when new frontend code
    is changed or pulled from upstream.

## Verification Commands

- `git diff --check`
- `GOCACHE=/Users/ray/new-api/.gocache GOTMPDIR=/Users/ray/new-api/.gotmp go test ./...`
- `cd web/classic && bun run build`
- `cd web/default && bun run build`
