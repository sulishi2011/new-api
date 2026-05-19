# Current Branch Change Record

Date: 2026-05-19

## Purpose

This document records the custom logic carried by `custom/main` so upstream
merges can be checked against it. During merges, preserve these behaviors unless
there is an explicit replacement with equivalent functionality.

## Branch And Baseline

- Custom branch: `custom/main`
- Upstream remote: `upstream` (`https://github.com/QuantumNous/new-api.git`)
- Custom baseline before the May 2026 upstream merge: `9f8a4ec05010da20704c1b55aa8b9af5630df72e`
- Local custom commits on top of that baseline:
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
- Relay trace capture:
  - Request headers, response headers, upstream request ID, upstream status, and
    bounded request/response body previews.
  - Binary and multipart bodies are represented by metadata instead of inline
    content.
  - Stream scanner idle timeout handling and trace capture must remain intact.
- Classic frontend:
  - Credential/provider-key page.
  - Dashboard filter and analysis table additions.
  - Usage-log trace viewer, provider-key filter, request-ID prefill, and expanded
    upstream metadata display.

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

### Monitoring And Operations

- Channel failure-rate monitoring:
  - `service/channel_health.go` failure-rate monitor.
  - Webhook notification payload/configuration changes.
  - Operation settings for failure-rate thresholds and notification behavior.
  - Classic monitoring settings UI.
- Usage aggregation:
  - Scheduled usage aggregation task registration in `main.go`.
  - `UsageAggregate` model/API/page.
  - Usage ledger integration remains compatible with aggregation.
  - Sidebar module and notification settings entries for usage aggregation remain
    available in the classic frontend.
- Database and runtime configuration:
  - Enhanced DB initialization error handling.
  - Docker compose customizations and `docker-compose.polo.yml`.
  - Tag/channel migration compatibility across SQLite, MySQL, and PostgreSQL.
  - Task polling and tag handling regressions covered by tests must stay fixed.

### Relay And Request DTO Semantics

- Optional request fields parsed from client JSON and re-marshaled upstream must
  preserve explicit zero/false values using pointer fields where needed.
- Message reasoning fields must preserve explicitly empty reasoning content.
- OpenAI image request/edit fields must preserve reference fields expected by
  upstream providers.
- Vertex URL building must support custom base URL / gateway prefix behavior.
- Relay HTTP client cache keys must include proxy and timeout policy where custom
  timeout behavior exists.

## Key Backend Files

- Controllers:
  - `controller/channel.go`
  - `controller/channel-billing.go`
  - `controller/channel-test.go`
  - `controller/channel_upstream_update.go`
  - `controller/log.go`
  - `controller/option.go`
  - `controller/provider_key.go`
  - `controller/relay.go`
  - `controller/usage_aggregate.go`
  - `controller/usedata.go`
  - `controller/vendor_profile.go`
- Models:
  - `model/channel.go`
  - `model/channel_cache.go`
  - `model/dashboard_usage.go`
  - `model/log.go`
  - `model/main.go`
  - `model/option.go`
  - `model/provider_key.go`
  - `model/provider_key_query.go`
  - `model/usage_aggregate.go`
  - `model/usage_ledger.go`
  - `model/vendor_profile.go`
- Relay/service:
  - `relay/common/trace_capture.go`
  - `relay/common/relay_info.go`
  - `relay/helper/stream_scanner.go`
  - `relay/channel/**`
  - `service/channel_health.go`
  - `service/http_client.go`
  - `service/log_info_generate.go`
  - `service/log_trace_capture.go`
  - `service/usage_aggregation_task.go`
- Routing/config:
  - `router/api-router.go`
  - `main.go`
  - `setting/operation_setting/monitor_setting.go`
  - `common/init.go`
  - `common/constants.go`

## Key Classic Frontend Files

After upstream's v1 frontend merge, these custom old-frontend files should live
under `web/classic/src`:

- `web/classic/src/App.jsx`
- `web/classic/src/components/dashboard/ChartsPanel.jsx`
- `web/classic/src/components/dashboard/DashboardAnalysisTable.jsx`
- `web/classic/src/components/dashboard/DashboardFilters.jsx`
- `web/classic/src/components/dashboard/DashboardHeader.jsx`
- `web/classic/src/components/dashboard/index.jsx`
- `web/classic/src/components/layout/PageLayout.jsx`
- `web/classic/src/components/layout/SiderBar.jsx`
- `web/classic/src/components/settings/OperationSetting.jsx`
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
- `web/classic/src/pages/UsageAggregate/index.jsx`
- `web/classic/src/i18n/locales/*.json`

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
7. Confirm optional relay DTO fields still preserve explicit zero/false values.
8. Run backend tests that cover custom models, relay, channel filtering, and
   aggregation.
9. Run classic frontend build; run default frontend build when new frontend code
   is changed or pulled from upstream.

## Verification Commands

- `git diff --check`
- `GOCACHE=/Users/ray/new-api/.gocache GOTMPDIR=/Users/ray/new-api/.gotmp go test ./...`
- `cd web/classic && bun run build`
- `cd web/default && bun run build`
