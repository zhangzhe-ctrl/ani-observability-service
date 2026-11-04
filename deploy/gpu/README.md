# GPU test environment collection

These manifests are opt-in for the verified three-node `ani-kcn-full` test environment. They are not installer defaults. Import the pinned image into each target node before applying the NVML manifest; no external NodePort or business GPU request is created.

- `nvml-test-environment.json`: NVML exporter v1.16.0, upstream source `104861909bb0ff46ecacf3ea9fab0f48d1903c6b`, OCI manifest `sha256:8bcaae69f55db00f8a215e30701df300712605de129aa706149eb3c214c8dca9`. The collector runs on all labelled GPU nodes at 15s intervals.
- `hami-test-environment.json`: 30s scrape of the existing Service selecting the monitor inside volcano-device-plugin Pods. It creates no monitor container and no additional feedback writer. Verify the Service endpoints, cache mount and running plugin Pods before applying.

Both profiles attach controlled cluster, node name, node UID and source labels. Verify the recorded node UIDs against Kubernetes and ACC before deployment and after node replacement. A replacement node requires a new verified UID; retaining an old UID would misattribute history. Missing labels fail the query's identity checks.

Physical GPU queries select the NVML profile. The dedicated business GPU queries read `ani_gpu_resource_memory_used_bytes` and `ani_gpu_resource_memory_limit_bytes`, recorded from the single controlled HAMi profile plus ACC current binding labels. Other legacy scrape jobs are not added to these sums. The HAMi profile retains only runtime memory usage and limit, and normalizes `deviceuuid` by removing the optional `GPU-` prefix and converting it to lowercase. NVML freshness remains 45s; the business rules accept HAMi and binding inputs up to 90s old. Business API sample timestamps are the actual recording-rule sample times, not raw hardware sampling times.

Take UID/config backups before applying or removing objects. The NVML DaemonSet uses `OnDelete`; updating its template does not itself replace existing Pods. Removing these task objects stops new samples and must not remove the existing HAMi Service or plugin. Backend Deployment rollback is separate. Prometheus storage, capacity and retention are owned by the environment operator and are not changed here.

## Current binding recording configuration (not applied)

The dedicated business GPU path is described in [gpu-current-bindings.md](../../docs/gpu-current-bindings.md). Add the two rules in `deployments/observability/collection/rules/ani-gpu-resource.rules.yaml`, or use the corresponding `ani-gpu-resource-metrics.yaml` PrometheusRule wrapper. The group runs every 30s and keeps `vdeviceid` separate; identical raw scrape identities are reduced with `max`, then actual slots are summed by the query.

`deployments/observability/collection/scrapes/acc-current-bindings.scrape.yaml` is a 15s template for the existing ACC `/metrics` target. The literal `ACC_EXISTING_ADMIN_METRICS_TARGET` is an operator-required deployment parameter; Prometheus will not expand it. Resolve the existing internal admin listener/Service/route, render the host and port, and check network access before applying. ACC defaults to loopback port 19091, so a loopback-only listener is not reachable from another Prometheus Pod. Keep access on the monitoring network and do not introduce a public endpoint or overwrite `ani_cluster_id` from ACC metrics.

These new rules, scrape changes, and fixtures are source changes only. They have not been applied or executed in this implementation task. Existing instance/KSM rules are unchanged; the deployment acceptance must confirm that normalizing HAMi `deviceuuid` leaves their value aggregation intact. Prometheus retention remains the environment's existing value; record it during the single cutover, without extending it for the API's 30-day maximum.
