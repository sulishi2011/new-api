# Log Trace S3 Storage And Log Retention Design

Date: 2026-05-19

## Goals

- Stop large request and response trace payloads from bloating the `logs` table.
- Keep recent request and response data available for debugging.
- Keep long-term billing, audit, and usage analytics safe.
- Make cleanup cheap for RDS by avoiding large updates to `logs.other`.
- Support SQLite, MySQL, and PostgreSQL in application code.

## Current State

Relay tracing is captured in memory as `relay/common.TracePayload`.
`service/log_info_generate.go` currently appends it to `other["trace"]`.
The final log row stores this inside `logs.other`.

This means each detailed request can put headers and body previews into the main
log row. Cleaning this later requires updating large `logs.other` values, which
creates RDS write amplification, WAL/binlog growth, row locks, and table bloat.

Existing cleanup only deletes whole log rows through:

- `controller.DeleteHistoryLogs`
- `model.DeleteOldLog`

There is no separate cleanup path for trace details.

## Target Architecture

Use three layers:

1. `logs`: long-term lightweight log index and accounting data.
2. `log_traces`: short-term lightweight trace metadata and S3 object reference.
3. S3: compressed request/response trace payloads.

`logs` must not store full `other.trace` for new records.

## Data Ownership

### `logs`

Keep fields needed for listing, filtering, accounting, auditing, and aggregate
queries:

- user, username, token, model, channel, group
- quota, cost_quota, tokens, latency
- request_id, external_request_id, upstream_request_id
- provider_key_id, vendor_profile_id, business attribution fields
- lightweight `other` metadata, without full `trace`

Recommended new index:

- `(type, created_at, id)` for type-specific retention deletes.

Existing indexes such as `(created_at, id)` and request-id indexes remain useful.

### `log_traces`

Create this table in `LOG_DB`, not the main application DB when `LOG_SQL_DSN` is
configured.

Recommended model:

```go
type LogTrace struct {
    Id                int    `json:"id"`
    LogId             int    `json:"log_id" gorm:"uniqueIndex"`
    CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
    ExpiresAt         int64  `json:"expires_at" gorm:"bigint;index"`
    Status            string `json:"status" gorm:"type:varchar(32);index"`
    StorageBackend    string `json:"storage_backend" gorm:"type:varchar(32);default:'s3'"`
    ObjectKey         string `json:"object_key" gorm:"type:varchar(512);index"`
    ObjectSize        int64  `json:"object_size" gorm:"bigint;default:0"`
    Compression       string `json:"compression" gorm:"type:varchar(16);default:'gzip'"`
    RequestBodySize   int64  `json:"request_body_size" gorm:"bigint;default:0"`
    ResponseBodySize  int64  `json:"response_body_size" gorm:"bigint;default:0"`
    RequestTruncated  bool   `json:"request_truncated" gorm:"default:false"`
    ResponseTruncated bool   `json:"response_truncated" gorm:"default:false"`
    RequestType       string `json:"request_content_type" gorm:"type:varchar(128);default:''"`
    ResponseType      string `json:"response_content_type" gorm:"type:varchar(128);default:''"`
    StatusCode        int    `json:"status_code" gorm:"default:0;index"`
    UpstreamRequestId string `json:"upstream_request_id" gorm:"type:varchar(128);index;default:''"`
    ErrorMessage      string `json:"error_message" gorm:"type:text"`
    CreatedTime       int64  `json:"created_time" gorm:"bigint"`
    UpdatedTime       int64  `json:"updated_time" gorm:"bigint"`
}
```

Do not store full request or response bodies in this table.

Do not enforce a database foreign key by default. `LOG_DB` may be separate, and
foreign keys add delete and migration pressure. Use `log_id` plus code-level
checks.

Recommended statuses:

- `pending`
- `stored`
- `upload_failed`
- `expired`
- `deleted`

### S3

Store one compressed JSON object per log trace.

Recommended object key:

```text
trace/yyyy/mm/dd/{log_id}-{request_id}.json.gz
```

If `request_id` is empty:

```text
trace/yyyy/mm/dd/{log_id}.json.gz
```

Recommended object payload:

```json
{
  "version": 1,
  "log_id": 123,
  "created_at": 1760000000,
  "request_id": "req_xxx",
  "external_request_id": "biz_xxx",
  "upstream_request_id": "upstream_xxx",
  "status_code": 200,
  "request": {
    "headers": {},
    "body": "...",
    "body_size": 1234,
    "content_type": "application/json",
    "truncated": false,
    "storage_kind": "inline_text"
  },
  "response": {
    "headers": {},
    "body": "...",
    "body_size": 4567,
    "content_type": "application/json",
    "truncated": true,
    "storage_kind": "inline_text"
  }
}
```

Use gzip compression first. Add zstd later only if dependency and runtime
support are acceptable.

## Configuration

Add environment/config options:

```text
TRACE_STORAGE_ENABLED=false
TRACE_STORAGE_BACKEND=s3
TRACE_RETENTION_DAYS=7
TRACE_CAPTURE_MODE=error
TRACE_SUCCESS_SAMPLE_RATE=0
TRACE_BODY_MAX_BYTES=65536
TRACE_UPLOAD_ASYNC=true
TRACE_UPLOAD_QUEUE_SIZE=1000
TRACE_UPLOAD_WORKERS=2
TRACE_UPLOAD_SYNC_FOR_ERROR=true
TRACE_UPLOAD_BATCH_ENABLED=false
TRACE_UPLOAD_BATCH_SIZE=100
TRACE_UPLOAD_BATCH_FLUSH_INTERVAL_SECONDS=2
TRACE_UPLOAD_BATCH_MAX_BYTES=8388608

TRACE_S3_BUCKET=
TRACE_S3_REGION=
TRACE_S3_ENDPOINT=
TRACE_S3_ACCESS_KEY_ID=
TRACE_S3_SECRET_ACCESS_KEY=
TRACE_S3_PREFIX=trace
TRACE_S3_FORCE_PATH_STYLE=false
TRACE_S3_SSE=
```

Capture modes:

- `off`: do not store traces.
- `error`: store traces for failed requests only.
- `sample`: store failed requests plus sampled successful requests.
- `all`: store all traces.

Batch upload mode:

- `TRACE_UPLOAD_BATCH_ENABLED=true` stores multiple trace archives in one gzip
  JSON object to reduce S3 PUT request cost.
- The request path only performs a non-blocking enqueue. S3 upload and
  `log_traces` metadata writes run in the background.
- If the in-memory queue is full, the API request is not blocked. That trace is
  dropped and a rate-limited system log is emitted.
- Admin trace lookup can lag by up to
  `TRACE_UPLOAD_BATCH_FLUSH_INTERVAL_SECONDS` when traffic is below
  `TRACE_UPLOAD_BATCH_SIZE`.
- Error traces are not synchronously uploaded in batch mode.

Default production recommendation:

```text
TRACE_CAPTURE_MODE=error
TRACE_RETENTION_DAYS=7
TRACE_SUCCESS_SAMPLE_RATE=0
TRACE_BODY_MAX_BYTES=65536
```

## Write Path

Current path:

```text
relay captures TracePayload
GenerateTextOtherInfo appends other.trace
RecordConsumeLog/RecordErrorLog writes logs.other
```

Target path:

```text
relay captures TracePayload
GenerateTextOtherInfo may keep TracePayload in memory
RecordConsumeLog/RecordErrorLog strips other.trace before LOG_DB.Create
LOG_DB.Create(log)
if trace should be stored:
  create log_traces row with status=pending
  upload compressed trace object to S3
  update log_traces status=stored and object metadata
```

Do not block normal relay success on S3 unless configured.

Recommended behavior:

- Success logs: async upload if sampled.
- Error logs: sync upload with short timeout when `TRACE_UPLOAD_SYNC_FOR_ERROR`
  is true; otherwise async.
- If S3 upload fails, keep the log row and mark `log_traces.upload_failed`.
- Never fail the API request only because trace upload failed.

Important: new `logs.other` should not contain `trace`. It may contain:

```json
{
  "trace_ref": true,
  "trace_status": "stored"
}
```

Prefer not updating `logs.other` after upload. The source of truth for status is
`log_traces`.

## Read Path

Add admin API:

```text
GET /api/log/:id/trace
```

Flow:

1. Load log row.
2. Check permissions.
3. Load `log_traces` by `log_id`.
4. If missing, expired, deleted, or upload failed, return a typed response.
5. Fetch S3 object through backend.
6. Decompress and return trace JSON.

Initial permission recommendation:

- Admin only.
- Do not expose trace bodies to normal users by default.

Optional later:

- User can view own trace only when a separate setting allows it.
- Redact headers and body fields before returning to non-admin users.

## S3 Lifecycle

Configure S3 lifecycle on the trace prefix:

```text
prefix: trace/
expiration: TRACE_RETENTION_DAYS + 1 day
abort incomplete multipart upload: 1 day
```

S3 lifecycle deletes object bodies cheaply without RDS updates.

The application cleanup job should still mark or delete expired `log_traces`
metadata.

## Log Retention

Trace cleanup solves large row pressure. It does not solve infinite `logs` row
growth. Add type-specific retention.

Recommended defaults:

```text
LOG_RETENTION_ENABLED=false
LOG_RETENTION_CONSUME_DAYS=90
LOG_RETENTION_ERROR_DAYS=90
LOG_RETENTION_SYSTEM_DAYS=180
LOG_RETENTION_MANAGE_DAYS=365
LOG_RETENTION_TOPUP_DAYS=0
LOG_RETENTION_REFUND_DAYS=0
LOG_CLEANUP_INTERVAL_HOURS=24
LOG_CLEANUP_BATCH_SIZE=300
LOG_CLEANUP_BATCH_SLEEP_MS=200
```

`0` means never auto-delete.

Do not delete `topup` and `refund` by default. They are financial audit records.

Delete algorithm:

```text
for each configured log type:
  cutoff = now - retention_days
  loop:
    ids = SELECT id FROM logs
          WHERE type = ? AND created_at < ?
          ORDER BY created_at, id
          LIMIT batch_size
    if ids empty: break
    DELETE FROM logs WHERE id IN ids
    sleep batch_sleep_ms
```

Use application-side select-then-delete instead of database-specific
`DELETE LIMIT` so SQLite, MySQL, and PostgreSQL stay compatible.

Before enabling deletion of consume logs, confirm usage aggregation is healthy.
Long-term usage analytics should come from `usage_ledger` and
`usage_aggregate_*`, not raw `logs`.

## Legacy Inline Trace Cleanup

Existing rows may already contain `logs.other.trace`.

There are two safe options:

1. Leave them until normal log retention deletes old rows.
2. Run an optional low-priority legacy cleanup job that removes only
   `other.trace` in small batches.

The legacy cleanup still updates `logs.other`, so it must be slower than normal
retention cleanup.

Recommended settings:

```text
LEGACY_TRACE_CLEANUP_ENABLED=false
LEGACY_TRACE_CLEANUP_OLDER_THAN_DAYS=7
LEGACY_TRACE_CLEANUP_BATCH_SIZE=100
LEGACY_TRACE_CLEANUP_SLEEP_MS=500
```

Implementation should parse JSON in Go and rewrite `other` without `trace`.
This avoids database-specific JSON SQL, but still creates row updates. Run it
only during low traffic windows.

## Migration Plan

### Phase 1: Schema and interfaces

- Add `model.LogTrace`.
- AutoMigrate `LogTrace` in `migrateLOGDB`.
- Add `storage/trace` service interface:
  - `PutTrace(ctx, key, payload)`.
  - `GetTrace(ctx, key)`.
  - `DeleteTrace(ctx, key)` optional.
- Add S3 implementation.
- Add in-memory or filesystem fake implementation for tests.

### Phase 2: Dual read

- Add `GET /api/log/:id/trace`.
- Read from `log_traces` first.
- If no `log_traces` row exists, optionally fall back to legacy
  `logs.other.trace` for old rows.

### Phase 3: New write path

- Strip `other.trace` before writing `logs`.
- Store trace payload in S3.
- Insert/update `log_traces`.
- Keep trace write failure non-fatal.

### Phase 4: Cleanup jobs

- Add `service/log_retention_task.go`.
- Add trace metadata expiration cleanup.
- Add type-specific log retention cleanup.
- Add optional legacy inline trace cleanup.

### Phase 5: UI

Classic frontend:

- Add trace storage status to log details.
- Add admin-only "view trace" action.
- Add log retention settings to operation log settings.
- Add manual cleanup buttons for:
  - expired trace metadata
  - legacy inline trace
  - old raw logs

Default frontend can follow later unless parity is required immediately.

## RDS Safety Rules

- No full-table updates.
- No large single transactions.
- Batch size starts at 100 to 300.
- Sleep between batches.
- Use `created_at` and `id` ordering.
- Prefer deleting independent `log_traces` rows over updating `logs`.
- Run legacy inline cleanup only off peak.
- Monitor:
  - RDS CPU
  - write IOPS
  - WAL/binlog size
  - replication lag
  - PostgreSQL dead tuples/autovacuum
  - slow queries on `logs`

## Failure Handling

- If S3 upload fails:
  - keep `logs` row
  - write or update `log_traces.status=upload_failed`
  - include error message truncated to a bounded size
- If S3 read returns not found:
  - mark `log_traces.status=expired` or `deleted`
  - return "trace expired"
- If cleanup fails:
  - log the error
  - retry next interval
  - do not block application startup

## Security And Redaction

Before uploading to S3, redact sensitive fields:

- `Authorization`
- `Proxy-Authorization`
- `Api-Key`
- `X-Api-Key`
- `Cookie`
- `Set-Cookie`
- provider keys in request bodies where known

Keep a configurable redaction list:

```text
TRACE_REDACT_HEADERS=authorization,proxy-authorization,api-key,x-api-key,cookie,set-cookie
TRACE_REDACT_JSON_KEYS=api_key,key,access_token,refresh_token,password,secret
```

Use SSE-S3 or SSE-KMS when available:

```text
TRACE_S3_SSE=AES256
```

## Testing

Backend tests:

- `LogTrace` AutoMigrate works on SQLite test DB.
- `RecordConsumeLog` no longer leaves `trace` inside `logs.other`.
- S3 storage fake receives a compressed trace object.
- `GET /api/log/:id/trace` enforces admin permission.
- Missing/expired trace returns a typed response.
- Log retention deletes only configured log types.
- `topup` and `refund` are not deleted when retention days are `0`.
- Batch cleanup stops correctly when fewer than batch size rows remain.

Manual verification:

- Create a successful request with capture mode `all`.
- Create a failed request with capture mode `error`.
- Confirm `logs.other` is small.
- Confirm `log_traces` row exists.
- Confirm S3 object exists and can be fetched by admin trace API.
- Run cleanup and confirm old S3 objects expire by lifecycle while `logs`
  remains queryable.

## Recommended Production Defaults

```text
TRACE_STORAGE_ENABLED=true
TRACE_STORAGE_BACKEND=s3
TRACE_CAPTURE_MODE=error
TRACE_SUCCESS_SAMPLE_RATE=0
TRACE_RETENTION_DAYS=7
TRACE_BODY_MAX_BYTES=65536
TRACE_UPLOAD_ASYNC=true
TRACE_UPLOAD_QUEUE_SIZE=1000
TRACE_UPLOAD_WORKERS=2
TRACE_UPLOAD_SYNC_FOR_ERROR=true
TRACE_UPLOAD_BATCH_ENABLED=false
TRACE_UPLOAD_BATCH_SIZE=100
TRACE_UPLOAD_BATCH_FLUSH_INTERVAL_SECONDS=2
TRACE_UPLOAD_BATCH_MAX_BYTES=8388608

LOG_RETENTION_ENABLED=true
LOG_RETENTION_CONSUME_DAYS=90
LOG_RETENTION_ERROR_DAYS=90
LOG_RETENTION_SYSTEM_DAYS=180
LOG_RETENTION_MANAGE_DAYS=365
LOG_RETENTION_TOPUP_DAYS=0
LOG_RETENTION_REFUND_DAYS=0
LOG_CLEANUP_BATCH_SIZE=300
LOG_CLEANUP_BATCH_SLEEP_MS=200
```

This keeps recent debugging data, protects financial records, and prevents the
main log table from growing without bound.
