# Business GPU monitoring from current bindings

The dedicated GPU API and GPU usage overview read Prometheus records carrying the business identity that ACC exported at recording time. ACC supplies only its current binding map. Observability does not query ACC scopes, retain an ownership history, reconstruct container exit evidence, or use monitoring availability to decide capacity.

This change is source-only until the coordinated deployment below. No tests, Prometheus fixtures, CI, builds, or deployment were run as part of this implementation task.

## Data path

The existing ACC `/metrics` handler exports `ani_acc_gpu_binding_info = 1` for current, active, fresh bindings whose signed UsageRef is valid. The exact labels are:

| Identity | Labels |
| --- | --- |
| Managed location | `ani_cluster_id`, `ani_node_id`, `ani_node_uid` |
| Runtime location | `podnamespace`, `podname`, `ctrname`, `pod_uid`, `deviceuuid` |
| Business identity | `ani_tenant_id`, `owner_service`, `resource_id`, `create_operation_id`, `resource_kind` |

There are 13 labels. `resource_kind` is `container` for `ani-resource-service`, `inference` for `ani-inference`, and `modeldev_execution` for `ani-modeldev-service`. The metric has no per-vdevice label: a current physical binding is one mapping. A collector read failure fails the scrape; it is not a successful empty map.

The existing controlled HAMi source supplies `vGPU_device_memory_usage_in_bytes` and `vGPU_device_memory_limit_in_bytes` with `ani_gpu_source="hami"`. The HAMi ServiceMonitor normalizes its existing `deviceuuid` label by removing the optional `GPU-` prefix and converting it to lowercase. ACC exports the same canonical value. Physical NVML `uuid` and its query path remain unchanged.

Two rules in [ani-gpu-resource.rules.yaml](../deployments/observability/collection/rules/ani-gpu-resource.rules.yaml) record:

- `ani_gpu_resource_memory_used_bytes`
- `ani_gpu_resource_memory_limit_bytes`

The group evaluates every 30 seconds. Each input must be no more than 90 seconds old and not in the future. Raw duplicate scrapes are reduced with `max` for the fixed cluster/node/Pod/container/device/vdevice/source identity. ACC replica duplicates are reduced with `max` for the full 13-label identity. The join preserves `vdeviceid` on each raw slot but excludes it from the join key, then attaches Pod UID and business labels. Multiple real slots and physical GPUs remain separate for the existing simultaneous sum.

Only these two business recording rules are added. There is no timestamp helper rule, second raw source, historical fallback, or sample-equality proof. Contradictory current identities for one join key are a collection/configuration error reported by rule evaluation; fix the source configuration. Monitoring is an observed usage view, not evidence for billing, admission, capacity, or hardware release.

The Operator wrapper is [ani-gpu-resource-metrics.yaml](../deployments/observability/collection/rules/ani-gpu-resource-metrics.yaml), with the existing `ani-observability` namespace and `release: ani-metrics` rule selector. Existing instance/KSM, training, VM, and legacy tenant-trend rules are unchanged.

## Existing ACC scrape target

[acc-current-bindings.scrape.yaml](../deployments/observability/collection/scrapes/acc-current-bindings.scrape.yaml) is a template for the existing Prometheus scrape configuration or additional-scrape configuration. It collects `/metrics` every 15 seconds with a 5-second scrape timeout and keeps the binding metric. The literal `${ACC_EXISTING_ADMIN_METRICS_TARGET}` is an operator-required deployment parameter, not a discovered address. Prometheus does not expand it automatically. When using an Operator additional-scrape secret, insert the template's `scrape_configs` list into the existing secret's expected list format.

Before rendering it, the deployment agent must inspect the existing ACC admin listener, internal Service or route, and network policy. ACC's checked-in default is `127.0.0.1:19091`; a loopback listener is not reachable from another Pod. Configure access through the existing monitoring network without creating a new endpoint, new Service, or public listener. The template does not set `ani_cluster_id`, `ani_node_id`, or `ani_node_uid`: a central ACC can report several managed clusters, and its metric labels must remain authoritative. The current task has not verified the actual deployed target or applied this template.

## Query contract

The gRPC method names, public GPU metric codes, range/latest request wrappers, and public URI shapes are unchanged. The internal target is now:

| Target kind | Required identity | Scopes |
| --- | --- | --- |
| `TENANT` | Authorized tenant UUID | Empty |
| `RESOURCE` | Tenant UUID, resource kind/id, full `usage_ref` tenant/owner/resource/create operation | Empty |
| `PHYSICAL` | Existing cluster/node/node UID/GPU identity | Existing physical scopes |

`GpuTarget.usage_ref` is field 11. Old input fields 6–10 are removed and reserved. `GpuSourceScope` retains fields 1, 2, 3, 14 and 15 for physical selection; historical business fields 4–13, 16 and 17 are reserved. `QueryGpuResponse.scope_watermark` field 9 is removed and reserved. Requests using business scopes or an incomplete resource UsageRef fail validation; there is no legacy fallback.

Governance retains resource authorization and resolves the original create operation. Only Governance's trusted mTLS identity can call these RPCs. Existing platform/tenant metadata checks remain, and the UsageRef tenant must agree with both the target and tenant context. Returned Prometheus labels must match the selected managed cluster and the authorized tenant/full resource reference.

Independent GPU queries use the configured managed Prometheus datasources. `QueryUsage` forwards its existing trusted `cluster_ids` subset through the internal `gpu.Query.ClusterIDs` field. Both are limited to 16 clusters; no public Prometheus URL or cluster selector is introduced. Existing concurrency, source union, sample count, body size, response size, and time-window budgets remain. GPU source budgets count actual returned records. A usage batch shares the GPU budget and reserves its CPU/memory budget under the same sample ceiling.

For business metrics the adapter uses the fixed recording selector plus `timestamp(record-selector)`. The timestamp is the actual recording-rule sample time, used for ordinary freshness and response metadata. Query evaluation time is not substituted for it. Physical NVML timestamps, gauges, Xid process boundaries, and event semantics are unchanged.

## Quality and history

The query sums returned sources at each 30-second base point and then calculates display-bucket statistics. It does not infer a source's lifetime from another timestamp or demand historical candidate, owner, or index evidence. Missing data does not become zero; an actual recorded zero remains zero.

| Response field | Meaning |
| --- | --- |
| `index_complete` | Every requested metric has returned values covering every requested base-grid bucket with ordinary freshness; this does not certify the full tenant workload inventory |
| `included_resource_kinds` | Distinct kinds on actual returned records |
| `uncovered_resource_kinds` | Empty array; it makes no promise about unobserved workloads |
| `metric_available_from` | First actual visible record time in the returned window |
| `ownership_available_from` | Deprecated compatibility field, never populated |
| `sample_time_min/max` | Actual recording sample times for business metrics; existing source times for physical metrics |
| `expected_sources` | Unset for business metrics; no expected workload universe is reconstructed |
| `observed_sources` | Actual visible source identities, including separate Pod UIDs and vdevice slots |
| `mean/peak` | Present only when the requested business time grid is complete |
| `known_value/known_peak`, usage `known_mean/known_peak` | Statistics of available samples, with missing time left absent |

No samples yields nullable values and `NO_DATA`. Partial time coverage yields `PARTIAL` and known statistics; dependencies failing remain query errors or usage `UNAVAILABLE`. An empty tenant enumeration or old empty scope inventory is not a measured GPU zero. CPU/memory usage collection retains its existing contract.

Once the new rules have written samples, deleting Pod A and rebuilding Pod B under the same full UsageRef leaves both segments queryable through that business identity. A new create operation is a different resource lifecycle; tenant history can still include both. The raw HAMi metric has no Pod UID, so association near a name replacement follows the current mapping with normal collection-period precision. Extremely short tasks and collection outages can have no samples.

Pre-cutover raw history has no complete business labels and is not backfilled. Preserve the deployment's existing Prometheus retention. Record its actual value during cutover; the API's 30-day maximum does not authorize extending storage. Requests exceeding the actual retention or preceding labeled records return `NO_DATA` or `PARTIAL` for the missing portion.

## Coordinated cutover and next-agent acceptance

Use one cutover window: prepare the new configurations and binaries; stop the old Governance/Observability entrypoints and ACC writers/agent; apply the ACC schema 9 migration; start the new ACC, Governance, and Observability versions with the two rules and rendered scrape config; then record `T0`, the first actual new labeled sample. Do not run old and new ACC production writers side by side or retain the old scope RPCs as a compatibility path.

The next validation/deployment agent must perform these acceptance cases before calling the path operational:

1. Run the changed Go tests and the new Prometheus fixture source. Check both raw rules and their Operator wrapper with the deployment's pinned tooling.
2. Verify the existing internal ACC target is reachable, its scrape succeeds, all 13 info labels are present, and no scrape label overwrites the metric's cluster identity.
3. Verify UUID normalization matches ACC and HAMi, preserves existing instance/KSM value aggregation, and preserves distinct `vdeviceid` values. Same-slot duplicate scrapes use their maximum; real slots sum.
4. Query both used and limit records for a tenant and a full resource reference. Reject a mismatched tenant, owner, resource, create operation, and business request containing legacy scopes.
5. Confirm actual zero returns zero, missing mapping/sample returns null, and input expiry yields `NO_DATA` without affecting ACC capacity or release.
6. Rebuild a resource into a new Pod UID. Its existing full UsageRef returns both post-`T0` segments; changing create operation separates resource history. No pre-`T0` ownership is manufactured.
7. Confirm partial and pre-retention windows leave full-window `mean/peak` absent, expose known statistics, and keep `index_complete` false. Confirm quality wording does not claim complete workload enumeration.
8. Confirm the ordinary physical GPU and exporter-event tests still pass. Record the existing retention value without changing it.

These are pending acceptance requirements, not claimed results of this implementation.
