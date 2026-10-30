package usage

import (
	"context"
	"errors"
	"time"

	gpuapi "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/gpu/v1"
	api "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/usage/v1"
	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/usage"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	api.UnimplementedResourceUsageServiceServer
	usecase *biz.UseCase
}

func New(u *biz.UseCase) *Service { return &Service{usecase: u} }
func (s *Service) QueryUsage(ctx context.Context, r *api.QueryUsageRequest) (*api.QueryUsageResponse, error) {
	if r == nil || r.Start == nil || r.End == nil || r.Start.CheckValid() != nil || r.End.CheckValid() != nil {
		return nil, rpcError(biz.Fail("USAGE_INVALID_QUERY", "valid start/end required"))
	}
	q := biz.Query{Start: r.Start.AsTime(), End: r.End.AsTime(), Clusters: r.ClusterIds, Metrics: r.Metrics}
	for _, t := range r.Tenants {
		if t == nil {
			return nil, rpcError(biz.Fail("USAGE_INVALID_QUERY", "nil tenant scope"))
		}
		q.Tenants = append(q.Tenants, biz.Tenant{ID: t.TenantId, ResourceID: t.ResourceTenantId, GPU: target(t.GpuTarget), GPUUnavailable: t.GpuUnavailableReason})
	}
	result, err := s.usecase.Execute(ctx, q)
	if err != nil {
		return nil, rpcError(err)
	}
	out := &api.QueryUsageResponse{Start: ts(result.Start), End: ts(result.End), DataCutoff: ts(result.Cutoff), StepSeconds: uint32(result.Step / time.Second)}
	for _, m := range result.Metrics {
		out.Metrics = append(out.Metrics, metric(m))
	}
	for _, t := range result.Tenants {
		row := &api.TenantUsage{TenantId: t.ID}
		for _, m := range t.Metrics {
			row.Metrics = append(row.Metrics, metric(m))
		}
		out.Tenants = append(out.Tenants, row)
	}
	if proto.Size(out) > 8*1024*1024 {
		return nil, rpcError(biz.Fail("USAGE_BUDGET_EXCEEDED", "response exceeds 8 MiB"))
	}
	return out, nil
}
func metric(m biz.Metric) *api.MetricUsage {
	out := &api.MetricUsage{Metric: m.Name, Unit: m.Unit, Mean: m.Mean, Peak: m.Peak, KnownMean: m.KnownMean, KnownPeak: m.KnownPeak, PeakAt: ts(m.PeakAt), ActiveTenantCount: m.ActiveTenants, Quality: &api.Quality{Status: m.Quality.Status, Reasons: m.Quality.Reasons, Coverage: m.Quality.Coverage, ObservedPoints: m.Quality.Observed, ExpectedPoints: m.Quality.Expected, AvailableFrom: ts(m.Quality.AvailableFrom)}}
	for _, p := range m.Points {
		out.Points = append(out.Points, &api.UsagePoint{Timestamp: ts(p.Timestamp), End: ts(p.End), Value: p.Value, KnownValue: p.Known, Peak: p.Peak})
	}
	return out
}
func target(t *gpuapi.GpuTarget) gpu.Target {
	if t == nil {
		return gpu.Target{}
	}
	out := gpu.Target{Kind: t.Kind, TenantID: t.TenantId, ResourceKind: t.ResourceKind, ResourceID: t.ResourceId, IndexComplete: t.IndexComplete, HistoryAvailableFrom: at(t.HistoryAvailableFrom), IncludedKinds: t.IncludedResourceKinds, UncoveredKinds: t.UncoveredResourceKinds, Watermark: t.ScopeWatermark}
	for _, s := range t.Scopes {
		if s == nil {
			out.IndexComplete = false
			continue
		}
		v := gpu.Scope{ClusterID: s.ClusterId, NodeID: s.NodeId, NodeUID: s.NodeUid, GPUUUID: s.GpuUuid, Namespace: s.Namespace, PodName: s.PodName, ContainerName: s.ContainerName, VDeviceID: s.VdeviceId, ValidFrom: at(s.ValidFrom), ValidTo: at(s.ValidTo), EvidenceAt: at(s.EvidenceAt), OwnershipState: s.OwnershipState, ResourceKind: s.ResourceKind, DeviceID: s.DeviceId, CandidateComplete: s.CandidateComplete, Reasons: s.Reasons}
		if s.UsageRef != nil {
			v.Usage = gpu.UsageRef{TenantID: s.UsageRef.TenantId, OwnerService: s.UsageRef.OwnerService, ResourceID: s.UsageRef.ResourceId, CreateOperationID: s.UsageRef.CreateOperationId}
		}
		out.Scopes = append(out.Scopes, v)
	}
	return out
}
func ts(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
func at(t *timestamppb.Timestamp) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.AsTime()
}
func rpcError(err error) error {
	reason := "USAGE_QUERY_FAILED"
	code := codes.Internal
	var f *biz.Failure
	if errors.As(err, &f) {
		reason = f.Reason
	}
	switch reason {
	case "USAGE_INVALID_QUERY":
		code = codes.InvalidArgument
	case "USAGE_BUDGET_EXCEEDED", "USAGE_CONCURRENCY_LIMIT":
		code = codes.ResourceExhausted
	case "USAGE_SOURCE_UNAVAILABLE":
		code = codes.Unavailable
	case "USAGE_SOURCE_RESPONSE_INVALID":
		code = codes.DataLoss
	}
	if errors.Is(err, context.DeadlineExceeded) {
		reason = "USAGE_QUERY_TIMEOUT"
		code = codes.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		reason = "USAGE_QUERY_CANCELED"
		code = codes.Canceled
	}
	st := status.New(code, reason)
	with, e := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: "ani-observability-service"})
	if e != nil {
		return st.Err()
	}
	return with.Err()
}
