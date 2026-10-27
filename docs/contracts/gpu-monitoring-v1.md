# Trusted GPU monitoring v1

Authority: GPU monitoring backend phase 1 v1.1, SHA256
`d599fb35d6ec179bc1f9b547f890465f36f7416a2e76a329cfa957aa42b451a7`.
The shared delivery index in Governance records live acceptance separately.

`observability.gpu.v1.GpuMonitoringService` is registered on the existing
TLS 1.3 Gov-only listener. Peer DNS SAN must be exactly `ani-governance`.
Every RPC requires `x-ani-actor=governance:user:<positive-id>` and UUID
`x-ani-request-id`. GPU additionally requires an explicit `x-ani-context`:

- `platform`: no tenant metadata; physical target or administrator tenant summary.
- `tenant`: valid `x-ani-tenant-id`, exactly matching the target; no physical target.

Legacy monitoring RPC metadata, source validation, time windows and deadlines
retain their existing contracts. GPU server deadline is 30 seconds; the caller
may impose a shorter one. Consumers need an 8 MiB receive message limit.

The physical/tenant/resource target and scopes are trusted internal inputs from
Gov/ACC. Tenant HTTP inputs must never supply them. Scopes carry immutable node
UID and controlled node name, normalized vendor UUID, ownership evidence,
original UsageRef and candidate completeness. Unknown or conflicting candidates
never contribute even to known partial totals. Per-grid completeness uses the
index start and active candidate completeness, so missing earlier history does
not invalidate normal samples after trustworthy indexing starts.

Controlled scrape profile labels are `ani_cluster_id` (configured datasource
UUID), `ani_node_id` (Kubernetes node name), `ani_node_uid` (immutable node UID),
and `ani_gpu_source=nvml|hami`. They must come from controlled discovery/relabeling.
NVML UUID is normalized lower case without `GPU-`; HAMi `deviceuuid` may carry
the prefix and either case. HAMi business selectors retain the raw
`podnamespace`, `podname`, `ctrname`, and `vdeviceid` dimensions. A missing trusted
vdevice selector is a wildcard; returned vdevice slots remain separate sources.
No raw GPU Pod UID is required. Duplicate scrape paths agree on value and
original sample time or become `SOURCE_CONFLICT`, never a sum of duplicates.

The controlled periods are NVML 15 seconds and HAMi 30 seconds. Original sample
timestamps are fetched using paired `timestamp(...)`, with freshness 45/90
seconds. Normal NVML collection additionally requires successful, recent
`nvidia_smi_last_collect_success[_timestamp_seconds]` evidence. Xid does not use
ordinary collection health, because its event loop is independent.

Queries use `[start,end)`, at most recent 30 days (one minute clock tolerance).
Resource grids are UTC epoch aligned 30 seconds; physical grids 15 seconds.
Display buckets are 30 seconds through 24 hours, 5 minutes through 7 days, then
15 minutes. The last bucket is truncated. All base points contribute before
bucket calculation. Resource and tenant peaks use simultaneous sums. Complete
summary means are weighted by complete base-point count. Known partial bucket
means use observed points; missing is null and valid zero remains zero. No
source means no observed value, never inferred zero.

Independent GPU limits: 4 metrics, 12 curves, 3000 buckets per curve, 12000 total
buckets, 128 independent source keys, 12 million processed values including
original-time and process-start companions, 6-hour chunks, 4 executing Prom
subrequests per request, 16 globally, 2 active queries per tenant (platform has
its own bucket), 10-second subrequest deadline, 30-second query deadline, 16 MiB
upstream body and 8 MiB final protobuf. Exceeding any limit fails the whole
query; no tail truncation or silent loss. Scope pagination must be bounded by
Gov before calling this RPC. Final per-event curves also consume the curve and
bucket budget.

Errors use `GPU_INVALID_QUERY` (InvalidArgument), `GPU_BUDGET_EXCEEDED` and
`GPU_CONCURRENCY_LIMIT` (ResourceExhausted), `GPU_SOURCE_NOT_CONFIGURED` and
`GPU_SOURCE_UNAVAILABLE` (Unavailable), `GPU_SOURCE_RESPONSE_INVALID` (DataLoss),
`GPU_QUERY_TIMEOUT` (DeadlineExceeded), and `GPU_QUERY_FAILED` (Internal), with
ErrorInfo domain `ani-observability-service`.

Optional ECC, remapped-row and Xid values are snapshots, not gauge bucket means
or interval counts. Xid returns per-code curves, exporter process start times,
and explicit process-lifetime/first-event/received-time semantics. Unsupported
is never inferred from absence; no series yields UNKNOWN/NO_DATA. Observed
valid values prove supported capability. Missing process evidence keeps Xid
only as a partial known snapshot. Real GPU faults are never manufactured by
the tests.

`metric_available_from` and `data_cutoff` are the earliest/latest original
sample times actually observed in this query window; they do not assert the
global first persisted sample or a retention guarantee.

Ordinary metric response summaries describe base-grid observations, not an
exact continuous-time signal or runtime duration. Scrape freshness proves no
HAMi internal cache measurement time. Actual history coverage and retention
capacity are external acceptance facts; query support alone proves neither.
