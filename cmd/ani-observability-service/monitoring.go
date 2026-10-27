package main

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/go-kratos/kratos/v3/middleware"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	gpuapi "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/gpu/v1"
	gpubiz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/monitoring"
	conf "github.com/zhangzhe-ctrl/ani-observability-service/internal/conf/v1"
	data "github.com/zhangzhe-ctrl/ani-observability-service/internal/data/monitoring"
	"github.com/zhangzhe-ctrl/ani-observability-service/internal/server"
	gpuservice "github.com/zhangzhe-ctrl/ani-observability-service/internal/service/gpu"
	service "github.com/zhangzhe-ctrl/ani-observability-service/internal/service/monitoring"
)

func openMonitoring(bc *conf.Bootstrap, logger *slog.Logger, middlewares ...middleware.Middleware) (*kratosgrpc.Server, func(), error) {
	m := bc.Monitoring
	if m == nil || m.PrometheusTimeout == nil || m.PrometheusTimeout.CheckValid() != nil || m.PrometheusTimeout.AsDuration() != 4*time.Second || m.MaxConcurrentRequests != 16 || m.MaxConcurrentRequestsPerTenant != 4 {
		return nil, nil, fmt.Errorf("monitoring requires fixed 4s budget and 16/4 concurrency limits")
	}
	configs := make([]data.Datasource, 0, len(m.Datasources))
	for _, source := range m.Datasources {
		if source == nil {
			return nil, nil, fmt.Errorf("nil monitoring datasource")
		}
		configs = append(configs, data.Datasource{ClusterID: source.ClusterId, Address: source.Address, BearerTokenFile: source.BearerTokenFile, TLSCAFile: source.TlsCaFile, TLSServerName: source.TlsServerName})
	}
	provider, err := data.New(configs)
	if err != nil {
		return nil, nil, err
	}
	cleanup := provider.Close
	usecase := biz.NewUseCase(provider, nil)
	listener, err := server.NewGovernanceServer(bc.Server.Grpc, bc.Governance, service.New(usecase, logger), middlewares...)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	gpuapi.RegisterGpuMonitoringServiceServer(listener, gpuservice.New(gpubiz.New(provider, nil)))
	return listener, cleanup, nil
}
