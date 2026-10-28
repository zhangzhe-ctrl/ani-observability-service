# Manual instance collection

The current collection profile was read from the approved `172.16.101.10–12`
cluster. The three minimal collector patches were applied at
`2026-10-19T08:37:16Z`. One KSM and three cAdvisor targets are up at 15s, and real
TSDB CPU series carry the confirmed cluster label. Normal authenticated Gov/Res
creation produced instance `90e2c65e-3601-44b6-9ef1-a7496c07a183`; its five raw
inputs, owner labels and UID/CID were verified. All eight rules are now loaded
and healthy; both canonical records first exist at `2026-10-19T09:17:55.589Z`.
Three-cycle verification passed with real exit code 0. OBS-08 subsequently
completed bounded load, same-UID CID restart, normal Gov stop/start with a new
Pod UID, and normal Gov deletion. The acceptance Pod is now absent, fresh
canonical CPU/memory are empty, and all three historical runtime identities
remain queryable. This GPU-spec instance proves CPU/memory collection on that
specification only; it does not claim CPU-spec or GPU metric acceptance.

Lifecycle evidence is retained under the remote task root's
`collection/lifecycle/`: `cid-exclusion-summary.json` proves the old CID's 5m
counter still existed while only the new CID was recorded;
`uid-exclusion-summary.json` proves the same exclusion across Pod UID replacement;
`deleted-final-snapshot-summary.json` proves the final Pod is absent, both fresh
records are empty, and each of the three identities can still be replayed at its
own original sample time. These are real source identities and raw recording
timestamps, not reconstructed business events.

`source-profile.yaml` records the real cluster identity, selected objects,
versions and source semantics. Resource's live `Service.ClusterID` is
`0fcdb8b1-22bf-4528-af59-82843dc88f21`. The KSM/cAdvisor source formats are real;
business identities in the promtool tests are synthetic.
`real-systemd-containerd-profile.json` is an existing non-Resource container and
proves source syntax only; `real-resource-workspace-profile.json` records the
normal Gov/Res-created workload's five actual source families. The
cgroupfs path has offline coverage; only systemd/containerd has live evidence.

All runtime commands below run through `ssh fedora`. The local workstation is
used for source editing and transfer. No Installer file, Helm release, global
selector, PVC or TSDB is modified. Prometheus tenant network isolation, writer
authentication and receiver restrictions were explicitly deferred by the user.
These changes do not claim that boundary is secure.

## Phase 2 increment: instance network rate metrics (2026-10-20)

The same rule group now records four additional rules. The two
`ani:instance:network_{receive,transmit}_source` helpers sit **before**
`ani:instance:binding` (same-group rules evaluate in file order, which keeps every
`timestamp(helper) == time()` snapshot guard satisfiable), and the two
`ani_instance_network_{receive,transmit}_bytes_per_second` canonicals are appended
last. The helpers compute `rate(...[5m])` over
`container_network_{receive,transmit}_bytes_total{interface="eth0"}`, extract only
`pod_uid` from the cgroup `id` (the `id` points at the pause sandbox, so
`runtime_container_id` and `container` are projected from the binding through the
4-key join `ani_cluster_id,namespace,pod,pod_uid`), and the canonical `sum by`
projects the same seven labels as CPU/memory. Unit is `bytes_per_second`.

Verified on ani-01: promtool `3.11.3` check (12 rules) and 41 fixtures PASS;
the applied PrometheusRule reports all 12 rules `health=ok` with no `lastError`;
each helper produces 112 current series, exactly matching the raw
`interface="eth0"` count, with `pod_uid` extracted and maximum sample age below
the 30s group interval. The canonicals are empty because no workspace instance
exists; end-to-end `verify.sh` acceptance stays deferred until one is created.
Pod-level semantics (sidecar traffic included), the conditional coverage wording
and the eth0/CNI re-verification contract are recorded in `source-profile.yaml`.

## Phase 3 increment: training execution metrics (2026-10-27)

A separate rule group `ani-training-metrics-v1` (8 rules, 30s) records
`ani_training_cpu_usage_cores` and `ani_training_memory_working_set_bytes` from
the same cAdvisor/KSM sources. The instance group is untouched. Training
selection requires the third Pod label `ani.io/workload-kind=trainer`, exposed
to KSM by a new patch pair (`ksm-workload-phase3.patch.json` /
`restore-ksm-workload-phase3.patch.json`); the phase 1 patches remain replayable
because each pair only covers one state transition.

Selection joins cAdvisor and KSM series with
`on(ani_cluster_id,namespace,pod)` to `kube_pod_labels{label_ani_io_workload_kind="trainer"}`,
so instance `workspace` containers never enter `ani_training_*`. Container names
are an external contract: worker `node`, step `main`; the step sidecar `wait` is
recorded but never selected by the query path. init containers exist only in
`kube_pod_init_container_info`, therefore they can never form a canonical — only
transient helper series while they run.

KSM rollout creates a 30-45s window without new samples and 45-90s of `NO_DATA`
for both instance and training records. Schedule it away from active executions.

## Phase 4 increment: tenant-scope resource trend (2026-10-28)

The tenant-level trend codes no longer read the instance recording rules. All
three curves now select the tenant's **entire workload set** from cAdvisor /
HAMi and join it to a tenant Pod view built from three label routes:

| Route | KSM source | Covered workloads |
| --- | --- | --- |
| ① | `kube_pod_labels{label_ani_io_tenant_id}` | container instances |
| ② | `kube_pod_labels{label_ani_kubercloud_com_tenant_id}` | inference / model pods in `ani-system` |
| ③ | `kube_namespace_labels{label_ani_io_resource_tenant_id}` | training namespaces (their Pods carry no tenant label) |

Route ③ requires a second KSM patch pair
(`ksm-tenant-scope.patch.json` / `restore-ksm-tenant-scope.patch.json`) that
extends the same `--metric-labels-allowlist` argument; the phase 3 pair stays
replayable because each pair covers exactly one state transition. Routes ① and ②
are mutually exclusive; the three routes are merged with an outer
`max by (namespace, pod)` because each carries a different `label_ani_*` set and
`or` does not deduplicate.

GPU switched from volcano vGPU share (`/ 100`, allocation) to
`vGPU_device_memory_usage_in_bytes` (HAMi actual usage, bytes, 90s freshness).
HAMi's own `namespace` is the device-plugin namespace, so `podnamespace` /
`podname` are projected onto `namespace` / `pod` before joining; duplicate
scrapes from the two ServiceMonitors are collapsed by
`max by (podnamespace, podname, ctrname, vdeviceid)`.

## Verified baseline and field changes

Prometheus `3.11.3`, Operator `0.90.1`, KSM `2.19.0`, Kubernetes `1.35.8`, and
containerd `2.3.4` are running in the existing cluster. The rule namespace selector
selects `ani-observability`; the rule label selector is `release: ani-metrics`.

| Object | Existing field | Task change |
| --- | --- | --- |
| Deployment/ani-metrics-kube-state-metrics | args contain port/resources, no label allowlist | append the two ANI Pod labels, preserve both old args |
| ServiceMonitor/ani-metrics-kube-state-metrics | endpoint 0 inherits 30s; no metric relabel | set 15s; append authoritative cluster label |
| ServiceMonitor/ani-metrics-kubelet | cAdvisor endpoint 1 uses 10s | set this endpoint to 15s; append authoritative cluster label after existing metric filters |
| PrometheusRule/ani-instance-metrics-v1 | absent at baseline | create the independently selected 30s group |

`honorTimestamps: true` and `trackTimestampsStaleness: true` already satisfy the
cAdvisor requirements and remain unchanged. Exposition samples have explicit
millisecond timestamps. CPU samples expose `cpu="total"`; no per-CPU sum is used.
Every helper retains the original source labels until uniqueness is proven.
Final `sum by` only projects a proven singleton to the seven canonical labels;
it never reduces multiple sources. Each helper consumer demands the current
group evaluation timestamp, so a lookback result cannot renew stale inputs.

## Evidence and access references

The task-owned remote evidence root is:

```text
/home/chabking/ani-observability-runs/phase1-20261019-01a15321
```

Original objects, exposition, five TSDB queries and targets are in `collection/`;
command/validation results are in `logs/collection/`. Keep those objects private.
The repository's `artifacts/` is ignored for subsequent local evidence copies.
Credential files are references only; never print or commit them.

The following variables are the verified Fedora tool and access references:

```bash
obs_root=/home/chabking/ani-observability-runs/phase1-20261019-01a15321
obs_collection="$obs_root/src/ani-observability-service/deployments/observability/collection"
obs_kubectl=/home/chabking/ani-four-service-deploy-20261012/tools/kubectl
export KUBECONFIG=/home/chabking/ani-four-service-deploy-20261012/private/kubeconfig
obs_context=kubernetes-admin@ani-lab
obs_namespace=ani-observability
```

## Read and back up before applying

First require the completed OBS-00 output and the normal Resource acceptance
instance's persisted scope. Re-read current objects. Each supplied JSON Patch
tests the original object UID and affected fields. If a test fails, save the new
object and reconcile the specific changed fields; do not remove the tests.

```bash
"$obs_kubectl" --context "$obs_context" get nodes -o wide
"$obs_kubectl" --context "$obs_context" config view --minify -o jsonpath='{.clusters[0].cluster.server}'
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" get prometheus ani-metrics-prometheus -o json > "$obs_root/collection/prometheus.before.json"
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" get deployment ani-metrics-kube-state-metrics -o json > "$obs_root/collection/ksm-workload.before.json"
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" get servicemonitor ani-metrics-kube-state-metrics -o json > "$obs_root/collection/ksm-servicemonitor.before.json"
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" get servicemonitor ani-metrics-kubelet -o json > "$obs_root/collection/cadvisor-servicemonitor.before.json"
```

Save the original `/api/v1/targets`, raw `/metrics/cadvisor` exposition and five
`/api/v1/query` responses. Existing examples are already retained in the evidence
root. An HTTP API vector's point time is evaluation time: use
`time()-timestamp(raw_selector)` to inspect actual original sample age.

## Check rules before applying

The target image supplies the same `promtool 3.11.3` as the running Prometheus.
It was pulled on Fedora from the existing internal registry; no global tool is
installed. These are ordinary Prometheus checks, run on the pure groups file.

```bash
podman run --rm --security-opt label=disable -v "$obs_collection:/work:ro" --workdir /work --entrypoint /bin/promtool 172.16.101.10:5000/prometheus/prometheus:v3.11.3-distroless check rules rules/ani-instance.rules.yaml
podman run --rm --security-opt label=disable -v "$obs_collection:/work:ro" --workdir /work --entrypoint /bin/promtool 172.16.101.10:5000/prometheus/prometheus:v3.11.3-distroless test rules tests/ani-instance.rules.test.yaml
```

The PrometheusRule's `spec.groups` is identical to the pure groups file. Verify
this when editing either file. Fixtures cover systemd and offline cgroupfs,
unknown paths, total/per-CPU selection, startup and reset, the inclusive 45s
boundary, every KSM input's freshness, snapshot expiry, duplicates, owner
conflicts/invalid IDs, UID/CID replacement, cross-cluster mismatch and retained
history. The one-ULP Prometheus comparison option preserves floating-point rate
semantics rather than rounding production values. All 30 fixtures passed on the
target-version promtool, including retention of two separate runtime identities.

## Apply the three field patches

Dry-run all three patches and the rule first. Review the resulting affected
fields. Do not load rules until real raw inputs pass the source checks.

```bash
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" patch deployment ani-metrics-kube-state-metrics --type=json --patch-file "$obs_collection/patches/ksm-workload.patch.json" --dry-run=server -o json > "$obs_root/collection/ksm-workload.dry-run.json"
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" patch servicemonitor ani-metrics-kube-state-metrics --type=json --patch-file "$obs_collection/patches/ksm-servicemonitor.patch.json" --dry-run=server -o json > "$obs_root/collection/ksm-servicemonitor.dry-run.json"
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" patch servicemonitor ani-metrics-kubelet --type=json --patch-file "$obs_collection/patches/cadvisor-servicemonitor.patch.json" --dry-run=server -o json > "$obs_root/collection/cadvisor-servicemonitor.dry-run.json"
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" apply --dry-run=server -f "$obs_collection/rules/ani-instance-metrics-v1.yaml"
```

Apply by removing only `--dry-run=server` from the first three commands, one at a
time. Wait for KSM after its args patch:

```bash
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" rollout status deployment/ani-metrics-kube-state-metrics --timeout=120s
```

KSM's rollout affects its own scrape availability. It does not restart business
Pods. Save each applied object as `*.after.json`. Check the reloaded targets are
up with 15s intervals. All five raw metric families must carry the authoritative
cluster label, and KSM must expose the actual Resource owner labels. Applying
these fields alone is not C02/C03 acceptance without a normal Resource instance.

When source validation passes, load the rule and save its UID:

```bash
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" apply -f "$obs_collection/rules/ani-instance-metrics-v1.yaml"
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" get prometheusrule ani-instance-metrics-v1 -o json > "$obs_root/collection/prometheusrule.after.json"
```

## Verify real canonical values

Use the actual persisted Resource tenant, instance and namespace as shell inputs;
the script intentionally has no example UUID fallback. A task-owned port-forward
can make Prometheus accessible on Fedora's loopback only:

```bash
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" port-forward svc/ani-metrics-prometheus --address=127.0.0.1 19098:9090
```

From another Fedora command, set `TENANT_ID`, `INSTANCE_ID`, `TARGET_NAMESPACE`
from Resource and run:

```bash
export KUBECTL="$obs_kubectl" CONTEXT="$obs_context"
export PROMETHEUS_URL=http://127.0.0.1:19098
export CLUSTER_ID=0fcdb8b1-22bf-4528-af59-82843dc88f21
export EVIDENCE_DIR="$obs_root/collection/verification-real"
"$obs_collection/verify.sh"
```

The script reads nodes, the actual Resource Pod, targets, rules, original sample
ages and both canonical series across three recording periods. It refuses
duplicate inputs and validates exactly seven labels. It never creates, deletes,
restarts or loads a business instance. Run `diagnostics.promql` at the recording
group's actual evaluation timestamp when checking the helper-current filters.
Empty canonical values cannot distinguish original conflicts from missing input;
retain the raw diagnostics. The coordinated OBS-08 run supplies bounded load,
CID/UID changes and stopped/deleted history tests through normal Resource
lifecycle calls. These scenarios are coordinated separately and are not
performed by the verifier.

## Restore only this task's fields

Restore is prepared. All three inverse patches passed server dry-run and their
returned `spec` exactly matched the saved original `spec`; actual restoration
has not been exercised. Re-read object UID and affected fields
before each command. The inverse patches test the exact applied field values.
Changes by another operator cause a failed patch; reconcile the conflict without
replacing entire old objects or arrays. Patches are atomic per object.

First remove this newly created rule only after its current UID equals the UID
saved in `prometheusrule.after.json`. If the object was replaced, stop this step.
Create the UID-precondition request body from the recorded UID. Its `dryRun`
must be in the `DeleteOptions` body. A live attempt using only URL
`?dryRun=All` unexpectedly deleted the newly created task-owned rule before it
had loaded; it was restored immediately. That attempt is recorded as failed,
and no further live delete dry-run has been executed. Inspect the current UID
before any intended restoration:

```bash
python3 - <<'PY'
import json, pathlib
p = pathlib.Path('/home/chabking/ani-observability-runs/phase1-20261019-01a15321/collection')
uid = json.loads((p / 'prometheusrule.after.json').read_text())['metadata']['uid']
(p / 'rule-delete-options.json').write_text(json.dumps({'apiVersion': 'v1', 'kind': 'DeleteOptions', 'dryRun': ['All'], 'preconditions': {'uid': uid}}) + '\n')
PY
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" get prometheusrule ani-instance-metrics-v1 -o jsonpath='{.metadata.uid}'
"$obs_kubectl" --context "$obs_context" delete --raw '/apis/monitoring.coreos.com/v1/namespaces/ani-observability/prometheusrules/ani-instance-metrics-v1' -f "$obs_root/collection/rule-delete-options.json"
```

Only when restoration is intended, remove the `dryRun` property from the request
body to delete this exact UID. This rule-deletion request remains prepared and
unverified; the three inverse collector patches below did pass server dry-run:

```bash
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" patch servicemonitor ani-metrics-kubelet --type=json --patch-file "$obs_collection/patches/restore-cadvisor-servicemonitor.patch.json" --dry-run=server -o json
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" patch servicemonitor ani-metrics-kube-state-metrics --type=json --patch-file "$obs_collection/patches/restore-ksm-servicemonitor.patch.json" --dry-run=server -o json
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" patch deployment ani-metrics-kube-state-metrics --type=json --patch-file "$obs_collection/patches/restore-ksm-workload.patch.json" --dry-run=server -o json
```

Apply each inverse patch by removing `--dry-run=server`, then wait for KSM and
verify the original targets. Rule removal leaves recorded TSDB history intact.
Do not Helm rollback, restore an old complete YAML, delete a namespace/PVC or
change unrelated collectors. Future Helm operations can overwrite these manual
fields; pass these exact artifacts to the later automation task.
