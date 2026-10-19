package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-kratos/kratos/v3/middleware"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	api "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/monitoring/v1"
	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"
	conf "github.com/zhangzhe-ctrl/ani-observability-service/internal/conf/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func NewGovernanceServer(c *conf.Server_GRPC, g *conf.Governance, service api.MonitoringServiceServer, middlewares ...middleware.Middleware) (*kratosgrpc.Server, error) {
	if g == nil || g.TlsCertFile == "" || g.TlsKeyFile == "" || g.ClientCaFile == "" {
		return nil, fmt.Errorf("governance TLS files are required")
	}
	pair, err := tls.LoadX509KeyPair(g.TlsCertFile, g.TlsKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load monitoring server certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse monitoring server certificate: %w", err)
	}
	identity := false
	for _, name := range leaf.DNSNames {
		if strings.Contains(name, "*") {
			return nil, fmt.Errorf("wildcard server identity is forbidden")
		}
		identity = identity || name == "ani-observability-service"
	}
	if !identity {
		return nil, fmt.Errorf("monitoring server DNS SAN mismatch")
	}
	pem, err := os.ReadFile(g.ClientCaFile)
	if err != nil {
		return nil, fmt.Errorf("read governance client CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("invalid governance client CA")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert}
	server := kratosgrpc.NewServer(kratosgrpc.Network(c.Network), kratosgrpc.Address(c.Addr), kratosgrpc.Timeout(c.Timeout.AsDuration()), kratosgrpc.Middleware(middlewares...), kratosgrpc.TLSConfig(tlsConfig), kratosgrpc.DisableReflection(), kratosgrpc.UnaryInterceptor(governanceIdentity), kratosgrpc.StreamInterceptor(func(any, grpc.ServerStream, *grpc.StreamServerInfo, grpc.StreamHandler) error {
		return status.Error(codes.PermissionDenied, "streaming RPC is not allowed")
	}))
	api.RegisterMonitoringServiceServer(server, service)
	return server, nil
}

func governanceIdentity(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if info.FullMethod != api.MonitoringService_QueryRange_FullMethodName && info.FullMethod != api.MonitoringService_QueryLatest_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "RPC is not allowed")
	}
	remote, ok := peer.FromContext(ctx)
	if !ok || remote == nil {
		return nil, status.Error(codes.Unauthenticated, "missing TLS peer")
	}
	tlsInfo, ok := remote.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.VerifiedChains) == 0 || len(tlsInfo.State.PeerCertificates) == 0 {
		return nil, status.Error(codes.Unauthenticated, "unverified TLS peer")
	}
	cert := tlsInfo.State.PeerCertificates[0]
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "ani-governance" || len(cert.IPAddresses) != 0 || len(cert.URIs) != 0 || len(cert.EmailAddresses) != 0 {
		return nil, status.Error(codes.PermissionDenied, "service identity is not allowed")
	}
	md, _ := metadata.FromIncomingContext(ctx)
	get := func(key string) string {
		values := md.Get(key)
		if len(values) != 1 {
			return ""
		}
		return values[0]
	}
	tenant, actor, requestID := get("x-ani-tenant-id"), get("x-ani-actor"), get("x-ani-request-id")
	if !target.ValidUUID(tenant) || !target.ValidUUID(requestID) || !validActor(actor) {
		return nil, status.Error(codes.Unauthenticated, "invalid trusted request metadata")
	}
	var targetTenant string
	switch request := req.(type) {
	case *api.QueryRangeRequest:
		targetTenant = request.GetTarget().GetTenantId()
	case *api.QueryLatestRequest:
		targetTenant = request.GetTarget().GetTenantId()
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid monitoring request")
	}
	if tenant != targetTenant {
		return nil, status.Error(codes.PermissionDenied, "target tenant mismatch")
	}
	return handler(ctx, req)
}

func validActor(actor string) bool {
	const prefix = "governance:user:"
	if !strings.HasPrefix(actor, prefix) {
		return false
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(actor, prefix), 10, 32)
	return err == nil && id != 0 && actor == prefix+strconv.FormatUint(id, 10)
}
