package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	api "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/monitoring/v1"
	targetapi "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/target/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	conf "github.com/zhangzhe-ctrl/ani-observability-service/internal/conf/v1"
)

func TestBuildAppRunsProductionComposition(t *testing.T) {
	grpcAddress := reserveAddress(t)
	adminAddress := reserveAddress(t)
	for adminAddress == grpcAddress {
		adminAddress = reserveAddress(t)
	}
	material := newRuntimeTLS(t)
	var queries atomic.Int32
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Errorf("unexpected Prometheus path %q", r.URL.Path)
		}
		queries.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
	}))
	defer prometheus.Close()
	bc := &conf.Bootstrap{Server: &conf.Server{
		Grpc:            &conf.Server_GRPC{Network: "tcp", Addr: grpcAddress, Timeout: durationpb.New(time.Second)},
		Admin:           &conf.Server_Admin{Network: "tcp", Addr: adminAddress, Timeout: durationpb.New(time.Second)},
		ShutdownTimeout: durationpb.New(3 * time.Second),
	}, Governance: material.governance(), Monitoring: runtimeMonitoring(prometheus.URL)}
	var logs bytes.Buffer
	app, err := buildApp(bc, newRuntimeLogger(&logs))
	if err != nil {
		t.Fatalf("buildApp() error = %v", err)
	}

	runResult := make(chan error, 1)
	go func() { runResult <- app.Run() }()
	waitForHTTP(t, "http://"+adminAddress+"/readyz")
	assertProductionAdmin(t, adminAddress)
	assertProductionMonitoring(t, grpcAddress, material.client)
	if queries.Load() != 2 {
		t.Fatalf("Prometheus queries = %d, want 2", queries.Load())
	}

	if err := app.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("production composition did not stop within its timeout")
	}
	if !strings.Contains(logs.String(), `"service.name":"ani-observability-service"`) {
		t.Fatalf("structured service identity missing from logs: %s", logs.String())
	}
	if !regexp.MustCompile(`"trace_id":"[0-9a-f]{32}"`).MatchString(logs.String()) {
		t.Fatalf("request trace correlation missing from logs: %s", logs.String())
	}
	if !regexp.MustCompile(`"span_id":"[0-9a-f]{16}"`).MatchString(logs.String()) {
		t.Fatalf("request span correlation missing from logs: %s", logs.String())
	}
}

func TestBuildAppRejectsInvalidConfig(t *testing.T) {
	_, err := buildApp(&conf.Bootstrap{}, newRuntimeLogger(io.Discard))
	if err == nil {
		t.Fatal("buildApp() unexpectedly accepted an incomplete config")
	}
}

func TestBuildAppKeepsAdminReadyWhenDatasourceIsOffline(t *testing.T) {
	material := newRuntimeTLS(t)
	grpcAddress, adminAddress, offlineAddress := reserveAddress(t), reserveAddress(t), reserveAddress(t)
	bc := &conf.Bootstrap{Server: &conf.Server{
		Grpc:            &conf.Server_GRPC{Network: "tcp", Addr: grpcAddress, Timeout: durationpb.New(6 * time.Second)},
		Admin:           &conf.Server_Admin{Network: "tcp", Addr: adminAddress, Timeout: durationpb.New(time.Second)},
		ShutdownTimeout: durationpb.New(3 * time.Second),
	}, Governance: material.governance(), Monitoring: runtimeMonitoring("http://" + offlineAddress)}
	app, err := buildApp(bc, newRuntimeLogger(io.Discard))
	if err != nil {
		t.Fatalf("offline datasource prevented startup: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	t.Cleanup(func() {
		if err := app.Stop(); err != nil {
			t.Errorf("stop offline-source app: %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("offline-source app: %v", err)
			}
		case <-time.After(4 * time.Second):
			t.Error("offline-source app did not stop")
		}
	})
	waitForHTTP(t, "http://"+adminAddress+"/readyz")
	connection, err := grpc.NewClient(grpcAddress, grpc.WithTransportCredentials(credentials.NewTLS(material.client)))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "x-ani-tenant-id", "5306af9c-c807-4aae-93ad-58294ec51458", "x-ani-actor", "governance:user:1", "x-ani-request-id", "c59b61f6-50f0-4bd4-aa03-885a331904ce")
	if _, err := api.NewMonitoringServiceClient(connection).QueryLatest(ctx, productionLatestRequest()); status.Code(err) != codes.Unavailable {
		t.Fatalf("offline datasource query = %v, want Unavailable", err)
	}
	waitForHTTP(t, "http://"+adminAddress+"/readyz")
}

func TestBuildAppRejectsMonitoringAndTLSConfigurationErrors(t *testing.T) {
	material := newRuntimeTLS(t)
	for _, tc := range []struct {
		name   string
		change func(*conf.Bootstrap)
	}{
		{"missing TLS", func(c *conf.Bootstrap) { c.Governance = nil }},
		{"unreadable key", func(c *conf.Bootstrap) { c.Governance.TlsKeyFile += ".missing" }},
		{"missing monitoring", func(c *conf.Bootstrap) { c.Monitoring = nil }},
		{"changed batch budget", func(c *conf.Bootstrap) { c.Monitoring.PrometheusTimeout = durationpb.New(time.Second) }},
		{"changed concurrency", func(c *conf.Bootstrap) { c.Monitoring.MaxConcurrentRequestsPerTenant = 5 }},
		{"invalid cluster", func(c *conf.Bootstrap) { c.Monitoring.Datasources[0].ClusterId = "not-a-uuid" }},
		{"duplicate cluster", func(c *conf.Bootstrap) {
			c.Monitoring.Datasources = append(c.Monitoring.Datasources, c.Monitoring.Datasources[0])
		}},
		{"missing address", func(c *conf.Bootstrap) { c.Monitoring.Datasources[0].Address = "" }},
		{"unreadable credentials", func(c *conf.Bootstrap) { c.Monitoring.Datasources[0].BearerTokenFile = "/missing/runtime-token" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &conf.Bootstrap{Server: &conf.Server{
				Grpc:            &conf.Server_GRPC{Network: "tcp", Addr: "127.0.0.1:19090", Timeout: durationpb.New(6 * time.Second)},
				Admin:           &conf.Server_Admin{Network: "tcp", Addr: "127.0.0.1:19091", Timeout: durationpb.New(time.Second)},
				ShutdownTimeout: durationpb.New(time.Second),
			}, Governance: material.governance(), Monitoring: runtimeMonitoring("http://127.0.0.1:1")}
			tc.change(c)
			if _, err := buildApp(c, newRuntimeLogger(io.Discard)); err == nil {
				t.Fatal("invalid startup configuration accepted")
			}
		})
	}
}

func reserveAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release reserved address: %v", err)
	}
	return address
}

func waitForHTTP(t *testing.T, endpoint string) {
	t.Helper()
	client := &http.Client{Timeout: 250 * time.Millisecond}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(endpoint)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("endpoint did not become ready: %s", endpoint)
}

func assertProductionAdmin(t *testing.T, address string) {
	t.Helper()
	response, err := (&http.Client{Timeout: time.Second}).Get("http://" + address + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read /healthz: %v", err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("GET /healthz = %d %q", response.StatusCode, body)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("GET /healthz content type = %q", contentType)
	}
}

func assertProductionMonitoring(t *testing.T, address string, tlsConfig *tls.Config) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		t.Fatalf("dial production gRPC: %v", err)
	}
	defer connection.Close()
	ctx = metadata.AppendToOutgoingContext(ctx,
		"x-ani-tenant-id", "5306af9c-c807-4aae-93ad-58294ec51458",
		"x-ani-actor", "governance:user:1",
		"x-ani-request-id", "c59b61f6-50f0-4bd4-aa03-885a331904ce")
	response, err := api.NewMonitoringServiceClient(connection).QueryLatest(ctx, productionLatestRequest())
	if err != nil {
		t.Fatalf("production monitoring RPC: %v", err)
	}
	if len(response.Series) != 2 || response.EvaluatedAt == nil {
		t.Fatalf("production monitoring response = %v", response)
	}
	for _, series := range response.Series {
		if series.Status != api.SeriesStatus_SERIES_STATUS_NO_DATA || len(series.Points) != 1 || series.Points[0].Value != nil || series.Points[0].GapReason != api.GapReason_GAP_REASON_NO_DATA {
			t.Fatalf("missing Prometheus data did not preserve business semantics: %v", series)
		}
	}
	_, err = grpc_health_v1.NewHealthClient(connection).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-monitoring health RPC = %v, want PermissionDenied", err)
	}
}

func productionLatestRequest() *api.QueryLatestRequest {
	return &api.QueryLatestRequest{
		Target: &targetapi.TargetScope{Kind: targetapi.TargetKind_TARGET_KIND_CONTAINER_INSTANCE,
			TenantId: "5306af9c-c807-4aae-93ad-58294ec51458", InstanceId: "81bf688c-aab1-451d-be81-40cadfbb7b35",
			ClusterId: "0fcdb8b1-22bf-4528-af59-82843dc88f21", Namespace: "runtime-test", ContainerName: "workspace"},
		MetricCodes: []string{"cpu.usage_cores", "memory.working_set_bytes"},
	}
}
