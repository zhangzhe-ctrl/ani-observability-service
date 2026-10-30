package server_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	gpuapi "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/gpu/v1"
	api "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/monitoring/v1"
	targetapi "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/target/v1"
	usageapi "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/usage/v1"
	gpubiz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/monitoring"
	usagebiz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/usage"
	conf "github.com/zhangzhe-ctrl/ani-observability-service/internal/conf/v1"
	data "github.com/zhangzhe-ctrl/ani-observability-service/internal/data/monitoring"
	server "github.com/zhangzhe-ctrl/ani-observability-service/internal/server"
	gpuservice "github.com/zhangzhe-ctrl/ani-observability-service/internal/service/gpu"
	service "github.com/zhangzhe-ctrl/ani-observability-service/internal/service/monitoring"
	usageservice "github.com/zhangzhe-ctrl/ani-observability-service/internal/service/usage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	reflection "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const trustedTenant = "5306af9c-c807-4aae-93ad-58294ec51458"
const trustedRequestID = "c59b61f6-50f0-4bd4-aa03-885a331904ce"

func TestGovernanceRealTLSAndMonitoringBusiness(t *testing.T) {
	ca := newTestCA(t)
	address := startGovernance(t, ca)
	clientTLS := ca.clientConfig(t, []string{"ani-governance"})
	connection := dialGovernance(t, address, clientTLS)
	ctx, cancel := context.WithTimeout(trustedContext(), 2*time.Second)
	defer cancel()
	response, err := api.NewMonitoringServiceClient(connection).QueryLatest(ctx, latestRequest())
	if err != nil {
		t.Fatalf("trusted monitoring query: %v", err)
	}
	if len(response.Series) != 2 || response.EvaluatedAt == nil {
		t.Fatalf("incomplete monitoring response: %v", response)
	}
	for _, series := range response.Series {
		if series.Status != api.SeriesStatus_SERIES_STATUS_NO_DATA || len(series.Points) != 1 || series.Points[0].Value != nil || series.Points[0].GapReason != api.GapReason_GAP_REASON_NO_DATA {
			t.Fatalf("empty external source must preserve real business missing-data semantics: %v", series)
		}
	}
	end := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	rangeResponse, err := api.NewMonitoringServiceClient(connection).QueryRange(ctx, &api.QueryRangeRequest{Target: latestRequest().Target, MetricCodes: []string{"cpu.usage_cores"}, Start: timestamppb.New(end.Add(-time.Minute)), End: timestamppb.New(end), StepSeconds: 30})
	if err != nil {
		t.Fatalf("trusted range query: %v", err)
	}
	if len(rangeResponse.Series) != 1 || len(rangeResponse.Series[0].Points) != 3 {
		t.Fatalf("range grid missing: %v", rangeResponse)
	}
}

func TestGovernanceRejectsInvalidPeerIdentity(t *testing.T) {
	ca := newTestCA(t)
	address := startGovernance(t, ca)
	for _, tc := range []struct {
		name      string
		configure func() *tls.Config
	}{
		{"no client certificate", func() *tls.Config { c := ca.clientConfig(t, nil); c.Certificates = nil; return c }},
		{"wrong CA", func() *tls.Config {
			other := newTestCA(t)
			c := other.clientConfig(t, []string{"ani-governance"})
			c.RootCAs = ca.roots
			return c
		}},
		{"same CA wrong identity", func() *tls.Config { return ca.clientConfig(t, []string{"ani-resource-service"}) }},
		{"multiple identities", func() *tls.Config { return ca.clientConfig(t, []string{"ani-governance", "another-service"}) }},
		{"wildcard identity", func() *tls.Config { return ca.clientConfig(t, []string{"*.ani-governance"}) }},
		{"wrong server identity", func() *tls.Config {
			c := ca.clientConfig(t, []string{"ani-governance"})
			c.ServerName = "ani-network-service"
			return c
		}},
		{"TLS 1.2", func() *tls.Config {
			c := ca.clientConfig(t, []string{"ani-governance"})
			c.MinVersion = tls.VersionTLS12
			c.MaxVersion = tls.VersionTLS12
			return c
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			connection := dialGovernance(t, address, tc.configure())
			ctx, cancel := context.WithTimeout(trustedContext(), time.Second)
			defer cancel()
			if _, err := api.NewMonitoringServiceClient(connection).QueryLatest(ctx, latestRequest()); err == nil {
				t.Fatal("untrusted peer unexpectedly reached monitoring")
			}
		})
	}
}

func TestGovernanceRejectsForgedMetadataAndMethods(t *testing.T) {
	ca := newTestCA(t)
	address := startGovernance(t, ca)
	connection := dialGovernance(t, address, ca.clientConfig(t, []string{"ani-governance"}))
	valid := metadata.Pairs("x-ani-tenant-id", trustedTenant, "x-ani-actor", "governance:user:1", "x-ani-request-id", trustedRequestID)
	for _, tc := range []struct {
		name, key string
		values    []string
		want      codes.Code
	}{
		{"missing tenant", "x-ani-tenant-id", nil, codes.Unauthenticated},
		{"duplicate tenant", "x-ani-tenant-id", []string{trustedTenant, trustedTenant}, codes.Unauthenticated},
		{"invalid tenant", "x-ani-tenant-id", []string{"00000000-0000-0000-0000-000000000000"}, codes.Unauthenticated},
		{"missing actor", "x-ani-actor", nil, codes.Unauthenticated},
		{"duplicate actor", "x-ani-actor", []string{"governance:user:1", "governance:user:1"}, codes.Unauthenticated},
		{"forged actor", "x-ani-actor", []string{"resource:user:1"}, codes.Unauthenticated},
		{"zero actor", "x-ani-actor", []string{"governance:user:0"}, codes.Unauthenticated},
		{"noncanonical actor", "x-ani-actor", []string{"governance:user:01"}, codes.Unauthenticated},
		{"overflow actor", "x-ani-actor", []string{"governance:user:4294967296"}, codes.Unauthenticated},
		{"missing request ID", "x-ani-request-id", nil, codes.Unauthenticated},
		{"duplicate request ID", "x-ani-request-id", []string{trustedRequestID, trustedRequestID}, codes.Unauthenticated},
		{"invalid request ID", "x-ani-request-id", []string{"not-a-uuid"}, codes.Unauthenticated},
		{"cross tenant admin scope", "x-ani-tenant-id", []string{"a63051b1-5544-4a8c-af4a-43a9316cd910"}, codes.PermissionDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md := valid.Copy()
			if len(tc.values) == 0 {
				md.Delete(tc.key)
			} else {
				md.Set(tc.key, tc.values...)
			}
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), time.Second)
			defer cancel()
			_, err := api.NewMonitoringServiceClient(connection).QueryLatest(ctx, latestRequest())
			if status.Code(err) != tc.want {
				t.Fatalf("query = %v, want %v", err, tc.want)
			}
		})
	}
	ctx, cancel := context.WithTimeout(trustedContext(), time.Second)
	defer cancel()
	if _, err := grpc_health_v1.NewHealthClient(connection).Check(ctx, &grpc_health_v1.HealthCheckRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("health RPC = %v", err)
	}
	if err := connection.Invoke(ctx, "/observability.monitoring.v1.MonitoringService/QueryArbitrary", latestRequest(), &api.QueryLatestResponse{}); status.Code(err) != codes.Unimplemented && status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unknown RPC = %v", err)
	}
	stream, err := reflection.NewServerReflectionClient(connection).ServerReflectionInfo(ctx)
	if err == nil {
		_ = stream.Send(&reflection.ServerReflectionRequest{MessageRequest: &reflection.ServerReflectionRequest_ListServices{ListServices: ""}})
		_, err = stream.Recv()
	}
	if status.Code(err) != codes.PermissionDenied && status.Code(err) != codes.Unimplemented {
		t.Fatalf("reflection RPC = %v", err)
	}
}

func TestGovernanceRejectsMissingTLSConfiguration(t *testing.T) {
	_, err := server.NewGovernanceServer(&conf.Server_GRPC{Network: "tcp", Addr: "127.0.0.1:0", Timeout: durationpb.New(6 * time.Second)}, nil, nil)
	if err == nil {
		t.Fatal("missing governance TLS accepted")
	}
}

func startGovernance(t *testing.T, ca *testCA) string {
	t.Helper()
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/query_range" {
			_, _ = io.WriteString(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
			return
		}
		_, _ = io.WriteString(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
	}))
	t.Cleanup(prometheus.Close)
	provider, err := data.New([]data.Datasource{{ClusterID: "0fcdb8b1-22bf-4528-af59-82843dc88f21", Address: prometheus.URL}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(provider.Close)
	certificate, key := ca.issue(t, []string{"ani-observability-service"}, x509.ExtKeyUsageServerAuth)
	certFile, keyFile := filepath.Join(t.TempDir(), "tls.crt"), filepath.Join(t.TempDir(), "tls.key")
	if err := os.WriteFile(certFile, certificate, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, key, 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := server.NewGovernanceServer(&conf.Server_GRPC{Network: "tcp", Addr: "127.0.0.1:0", Timeout: durationpb.New(6 * time.Second)}, &conf.Governance{TlsCertFile: certFile, TlsKeyFile: keyFile, ClientCaFile: ca.path}, service.New(biz.NewUseCase(provider, nil), slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err != nil {
		t.Fatal(err)
	}
	gpuapi.RegisterGpuMonitoringServiceServer(listener, gpuservice.New(gpubiz.New(provider, nil)))
	usageapi.RegisterResourceUsageServiceServer(listener, usageservice.New(usagebiz.New(gpubiz.New(provider, nil), provider, nil)))
	endpoint, err := listener.Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- listener.Start(context.Background()) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := listener.Stop(ctx); err != nil {
			t.Errorf("stop governance listener: %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("governance listener: %v", err)
			}
		case <-ctx.Done():
			t.Error("governance listener did not terminate")
		}
	})
	return endpoint.Host
}

func latestRequest() *api.QueryLatestRequest {
	return &api.QueryLatestRequest{Target: &targetapi.TargetScope{Kind: targetapi.TargetKind_TARGET_KIND_CONTAINER_INSTANCE, TenantId: trustedTenant, InstanceId: "81bf688c-aab1-451d-be81-40cadfbb7b35", ClusterId: "0fcdb8b1-22bf-4528-af59-82843dc88f21", Namespace: "runtime-test", ContainerName: "workspace"}, MetricCodes: []string{"cpu.usage_cores", "memory.working_set_bytes"}}
}
func trustedContext() context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-ani-tenant-id", trustedTenant, "x-ani-actor", "governance:user:1", "x-ani-request-id", trustedRequestID))
}
func dialGovernance(t *testing.T, address string, c *tls.Config) *grpc.ClientConn {
	t.Helper()
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(c)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return connection
}

type testCA struct {
	certificate *x509.Certificate
	key         *ecdsa.PrivateKey
	roots       *x509.CertPool
	path        string
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: randomSerial(t), Subject: pkix.Name{CommonName: "governance test CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(encoded)
	path := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return &testCA{certificate: certificate, key: key, roots: roots, path: path}
}
func (ca *testCA) issue(t *testing.T, names []string, usage x509.ExtKeyUsage) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: randomSerial(t), DNSNames: names, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, ca.certificate, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})
}
func (ca *testCA) clientConfig(t *testing.T, names []string) *tls.Config {
	t.Helper()
	certificate, key := ca.issue(t, names, x509.ExtKeyUsageClientAuth)
	pair, err := tls.X509KeyPair(certificate, key)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, ServerName: "ani-observability-service", RootCAs: ca.roots, Certificates: []tls.Certificate{pair}}
}
func randomSerial(t *testing.T) *big.Int {
	t.Helper()
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGpuExplicitPlatformAndTenantContexts(t *testing.T) {
	ca := newTestCA(t)
	address := startGovernance(t, ca)
	connection := dialGovernance(t, address, ca.clientConfig(t, []string{"ani-governance"}))
	client := gpuapi.NewGpuMonitoringServiceClient(connection)
	req := &gpuapi.QueryGpuLatestRequest{Target: &gpuapi.GpuTarget{Kind: "PHYSICAL", IndexComplete: true, Scopes: []*gpuapi.GpuSourceScope{{ClusterId: "0fcdb8b1-22bf-4528-af59-82843dc88f21", NodeId: "node1", NodeUid: "node-uid", GpuUuid: "abc"}}}, MetricCodes: []string{"gpu.physical.memory_used_bytes"}}
	platform := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-ani-context", "platform", "x-ani-actor", "governance:user:1", "x-ani-request-id", trustedRequestID))
	response, err := client.QueryGpuLatest(platform, req)
	if err != nil {
		t.Fatal(err)
	}
	if response.GetResult().GetSeries()[0].GetQuality().GetStatus() != "NO_DATA" {
		t.Fatal(response)
	}
	if _, err := client.QueryGpuLatest(trustedContext(), req); status.Code(err) != codes.Unauthenticated {
		t.Fatal("missing explicit context", err)
	}
	tenant := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-ani-context", "tenant", "x-ani-tenant-id", trustedTenant, "x-ani-actor", "governance:user:1", "x-ani-request-id", trustedRequestID))
	if _, err := client.QueryGpuLatest(tenant, req); status.Code(err) != codes.PermissionDenied {
		t.Fatal("tenant cannot read physical", err)
	}
	platformWithTenant := metadata.AppendToOutgoingContext(platform, "x-ani-tenant-id", trustedTenant)
	if _, err := client.QueryGpuLatest(platformWithTenant, req); status.Code(err) != codes.PermissionDenied {
		t.Fatal("fake platform tenant", err)
	}
}

func TestResourceUsageRequiresPlatformTLSContextAndNoTenantHeader(t *testing.T) {
	ca := newTestCA(t)
	address := startGovernance(t, ca)
	connection := dialGovernance(t, address, ca.clientConfig(t, []string{"ani-governance"}))
	client := usageapi.NewResourceUsageServiceClient(connection)
	end := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	request := &usageapi.QueryUsageRequest{Start: timestamppb.New(end.Add(-time.Minute)), End: timestamppb.New(end), ClusterIds: []string{"0fcdb8b1-22bf-4528-af59-82843dc88f21"}, Metrics: []string{"gpu"}, Tenants: []*usageapi.TenantScope{{TenantId: 1, ResourceTenantId: trustedTenant, GpuUnavailableReason: "HISTORY_UNAVAILABLE"}}}
	platform := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-ani-context", "platform", "x-ani-actor", "governance:user:1", "x-ani-request-id", trustedRequestID))
	result, err := client.QueryUsage(platform, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Metrics) != 1 || result.Metrics[0].Mean != nil || result.Tenants[0].Metrics[0].Quality.Status != "UNAVAILABLE" {
		t.Fatalf("missing trusted observation fabricated value: %v", result)
	}
	for _, ctx := range []context.Context{trustedContext(), metadata.AppendToOutgoingContext(platform, "x-ani-tenant-id", trustedTenant), metadata.AppendToOutgoingContext(platform, "x-ani-tenant-id", "")} {
		if _, err := client.QueryUsage(ctx, request); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("tenant or forged platform context reached usage RPC: %v", err)
		}
	}
	wrong := dialGovernance(t, address, ca.clientConfig(t, []string{"ani-resource-service"}))
	if _, err := usageapi.NewResourceUsageServiceClient(wrong).QueryUsage(platform, request); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-Gov peer reached usage RPC: %v", err)
	}
}
