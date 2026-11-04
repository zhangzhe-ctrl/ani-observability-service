# Trusted GPU monitoring v1

Authority: GPU monitoring backend phase 1 v1.1, SHA256
`d599fb35d6ec179bc1f9b547f890465f36f7416a2e76a329cfa957aa42b451a7`.
The current-binding revision replaces the business scope/index contract below; the physical contract remains. See [current binding monitoring](../gpu-current-bindings.md) for the exact wire fields, rules, and coordinated cutover. The shared delivery index in Governance records live acceptance separately; this revision has not been deployed or tested in this implementation task.

`observability.gpu.v1.GpuMonitoringService` is registered on the existing
TLS 1.3 Gov-only listener. Peer DNS SAN must be exactly `ani-governance`.
Every RPC requires `x-ani-actor=governance:user:<positive-id>` and UUID
`x-ani-request-id`. GPU additionally requires an explicit `x-ani-context`:

- `platform`: no tenant metadata; physical target or administrator tenant summary.
- `tenant`: valid `x-ani-tenant-id`, exactly matching the target; no physical target.

Legacy monitoring RPC metadata, source validation, time windows and deadlines
retain their existing contracts. GPU server deadline is 30 seconds; the caller
may impose a shorter one. Consumers need an 8 MiB receive message limit.

Gov supplies authorized direct identities: TENANT carries its tenant UUID, RESOURCE carries the full tenant/owner/resource/create-operation UsageRef, and PHYSICAL retains trusted cluster/node/node UID/GPU scopes. Business scopes are rejected. The query does not use historical ownership evidence, candidate completeness, an index start, or source lifetimes.

Controlled scrape labels remain `ani_cluster_id`, `ani_node_id`, `ani_node_uid`, and `ani_gpu_source=nvml|hami`. Physical NVML UUID remains lowercase without `GPU-`. The existing controlled HAMi profile now normalizes `deviceuuid` the same way. Its raw memory usage/limit values are joined with `ani_acc_gpu_binding_info` at recording time to produce `ani_gpu_resource_memory_used_bytes` and `ani_gpu_resource_memory_limit_bytes`. The records retain Pod UID, full business identity, and separate vdevice slots. Raw duplicate scrape identities use `max` in the rules; business aggregation does not require duplicate value/time equality. Physical source conflict handling remains unchanged.

The controlled periods are NVML 15 seconds and HAMi 30 seconds; the binding scrape is 15 seconds and business rules run every 30 seconds. Ordinary freshness is 45/90 seconds. Paired `timestamp(...)` queries retain actual sample times: raw NVML time for physical metrics and recording-rule time for business metrics. Normal NVML collection additionally requires successful, recent `nvidia_smi_last_collect_success[_timestamp_seconds]` evidence. Xid does not use ordinary collection health, because its event loop is independent.

Queries use `[start,end)`, at most recent 30 days (one minute clock tolerance).
Resource grids are UTC epoch aligned 30 seconds; physical grids 15 seconds.
Display buckets are 30 seconds through 24 hours, 5 minutes through 7 days, then
15 minutes. The last bucket is truncated. All base points contribute before
bucket calculation. Resource and tenant peaks use simultaneous sums. Business summary mean/peak require complete coverage of the requested base grid. Known partial bucket
means use observed points; missing is null and valid zero remains zero. No
source means no observed value, never inferred zero.

Independent GPU limits: 16 managed clusters, 4 metrics, 12 curves, 3000 buckets per curve, 12000 total
buckets, 128 independent source keys, 12 million processed values including
original-time and process-start companions, 6-hour chunks, 4 executing Prom
subrequests per request, 16 globally, 2 active queries per tenant (platform has
its own bucket), 10-second subrequest deadline, 30-second query deadline, 16 MiB
upstream body and 8 MiB final protobuf. Exceeding any limit fails the whole
query; no tail truncation or silent loss. Business queries use configured managed sources, or the trusted QueryUsage cluster subset; there is no ACC scope pagination. Final per-event curves also consume the curve and bucket budget.

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

`metric_available_from` and `data_cutoff` are the earliest/latest actual sample times visible in the query window, using recording sample time for business metrics. They do not assert the global first persisted sample or a retention guarantee. `ownership_available_from` is deprecated and never populated. `index_complete` describes returned time-grid coverage only, `included_resource_kinds` comes from returned labels, and `uncovered_resource_kinds` is empty. Business `expected_sources` is unset; no workload universe is inferred.

Ordinary metric response summaries describe base-grid observations, not an
exact continuous-time signal or runtime duration. Scrape freshness proves no
HAMi internal cache measurement time. Actual history coverage and retention
capacity are external acceptance facts; query support alone proves neither.

Pre-cutover unlabeled raw history is not backfilled. Keep existing Prometheus retention and record its actual value; the API maximum of 30 days does not expand retention. No sample, unavailable source, or empty old scope becomes a measured zero. Monitoring failures never block ACC capacity or retirement.
