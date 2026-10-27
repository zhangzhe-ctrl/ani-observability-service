# GPU test environment collection

These manifests are opt-in for the verified three-node `ani-kcn-full` test environment. They are not installer defaults. Import the pinned image into each target node before applying the NVML manifest; no external NodePort or business GPU request is created.

- `nvml-test-environment.json`: NVML exporter v1.16.0, upstream source `104861909bb0ff46ecacf3ea9fab0f48d1903c6b`, OCI manifest `sha256:8bcaae69f55db00f8a215e30701df300712605de129aa706149eb3c214c8dca9`. The collector runs on all labelled GPU nodes at 15s intervals.
- `hami-test-environment.json`: 30s scrape of the existing Service selecting the monitor inside volcano-device-plugin Pods. It creates no monitor container and no additional feedback writer. Verify the Service endpoints, cache mount and running plugin Pods before applying.

Both profiles attach controlled cluster, node name, node UID and source labels. Verify the recorded node UIDs against Kubernetes and ACC before deployment and after node replacement. A replacement node requires a new verified UID; retaining an old UID would misattribute history. Missing labels fail the query's identity checks.

GPU queries select these source profiles. Other legacy scrape jobs are not added to the same NVML/HAMi series sums. The HAMi profile retains only runtime memory usage and limit; physical metrics use NVML. NVML freshness is 45s and HAMi freshness is 90s, based on original sample timestamps.

Take UID/config backups before applying or removing objects. The NVML DaemonSet uses `OnDelete`; updating its template does not itself replace existing Pods. Removing these task objects stops new samples and must not remove the existing HAMi Service or plugin. Backend Deployment rollback is separate. Prometheus storage, capacity and retention are owned by the environment operator and are not changed here.
