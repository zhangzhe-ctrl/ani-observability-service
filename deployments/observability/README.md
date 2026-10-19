# Observability phase 1 deployment

The live target is the 10–12 cluster, context `kubernetes-admin@ani-lab`.
Its Res cluster ID is `0fcdb8b1-22bf-4528-af59-82843dc88f21`.
This service queries `http://ani-metrics-prometheus.ani-observability.svc:9090`.
Collection and recording rules are managed separately in [collection](collection/README.md).

Build on the authorized Fedora host with Go 1.26.9 and published API dependencies:

```sh
OBS_VERSION=$(git describe --tags --match 'v[0-9]*' --always --dirty)
GOWORK=off CGO_ENABLED=0 make verify build VERSION="$OBS_VERSION"
podman build --pull=never --network=none -t 172.16.101.10:5000/ani-observability-service:phase1-20261019-$OBS_VERSION .
podman save --format oci-archive -o ani-observability-service.oci.tar 172.16.101.10:5000/ani-observability-service:phase1-20261019-$OBS_VERSION
```

The pinned distroless image contains the compiled static binary and default configuration.
The deployment mounts the reviewed live ConfigMap instead of relying on image defaults.
For a private HTTPS datasource, mount its explicit CA file and set `tls_ca_file`.
The existing registry was verified read-only (push returns 405); deliver the OCI
archive through the authorized task-owned node image loader and record the
imported image digest. Do not retry pushes or change registry configuration.
The commands above select the actual checked-out service version. The initial
candidate deployed during integration used OCI digest
`sha256:98f89b379e9375dcb665b37d409a4e0e332876e77093bbb6ebab72fc87153a6c`.
OCI export can change the source image manifest digest; always use the archive
manifest and the imported containerd reference in the Deployment.
Final publication versions and their replacement digest are recorded by OBS-09.

The existing cert-manager `ClusterIssuer/ani-ca` issues the two task-owned
certificates in `certificates.yaml`. Neither CA private keys nor leaf private
keys belong in Git or task reports. The server identity is the literal DNS SAN
`ani-observability-service`; its TLS listener requires TLS 1.3 and a CA-verified
client certificate whose only identity is DNS SAN `ani-governance`.
Renewed leaf files require a rollout because certificates load at startup.

Apply and inspect using the actual authorized kubeconfig:

```sh
kubectl apply -f deployments/observability/certificates.yaml
kubectl -n ani-observability wait --for=condition=Ready certificate/ani-observability-server-tls --timeout=60s
kubectl -n ani-system wait --for=condition=Ready certificate/ani-observability-governance-client-tls --timeout=60s
kubectl apply -f deployments/observability/configmap.yaml
kubectl apply -f deployments/observability/service.yaml
kubectl apply -f deployments/observability/deployment.yaml
kubectl -n ani-observability rollout status deployment/ani-observability-service --timeout=60s
```

Gov connects to `ani-observability-service.ani-observability.svc:19090` and
verifies server name `ani-observability-service`. Mount its client Secret
`ani-system/ani-observability-governance-client-tls` read-only and set:

```text
ANI_OBSERVABILITY_GRPC_ADDR=ani-observability-service.ani-observability.svc:19090
ANI_OBSERVABILITY_CA_FILE=<mount>/ca.crt
ANI_OBSERVABILITY_CLIENT_CERT_FILE=<mount>/tls.crt
ANI_OBSERVABILITY_CLIENT_KEY_FILE=<mount>/tls.key
```

Only `/observability.monitoring.v1.MonitoringService/QueryRange` and
`/observability.monitoring.v1.MonitoringService/QueryLatest` accept calls.
Each requires one canonical `x-ani-tenant-id`, one `x-ani-actor` of the form
`governance:user:<canonical uint32 user ID>`, and one nonzero canonical UUID
`x-ani-request-id`. The target tenant must match the trusted metadata.
Public gRPC health, reflection, and streaming methods are rejected.
Admin `/healthz`, `/readyz`, and `/metrics` remain on port 19091, which is not
published through this Service. The Deployment uses admin HTTP probes.
No Kubernetes API access, business database, queue, or persistent volume is used.

Prometheus bottom-level tenant isolation and expired-tenant product policy are
deferred by [the execution override](../../docs/specs/observability-phase1/EXECUTION-OVERRIDES.md).
The current CNI has no NetworkPolicy enforcement. This delivery adds no
NetworkPolicy, Prometheus authentication proxy, or Prometheus mTLS.
Do not describe a private ClusterIP as proof of bottom-level tenant isolation.

Rollback only these task-owned Deployment, Service, ConfigMap and Certificates
after checking that no accepted consumer still needs them. Do not delete
Prometheus, monitoring releases, existing certificates, PVCs, or TSDB data.
Actual image digest, apply output and runtime verification are in the OBS-06
report; these manifests alone do not prove deployment or business acceptance.
