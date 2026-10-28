package gpu

import (
	"context"
	"errors"
	"time"

	api "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/gpu/v1"
	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	api.UnimplementedGpuMonitoringServiceServer
	usecase *biz.UseCase
}

func New(u *biz.UseCase) *Service { return &Service{usecase: u} }
func (s *Service) QueryGpuLatest(ctx context.Context, r *api.QueryGpuLatestRequest) (*api.QueryGpuLatestResponse, error) {
	if r == nil {
		return nil, rpcError(biz.Fail("GPU_INVALID_QUERY", errors.New("request required")))
	}
	v, e := s.execute(ctx, biz.Query{Target: target(r.Target), Codes: r.MetricCodes, Latest: true})
	if e != nil {
		return nil, e
	}
	return &api.QueryGpuLatestResponse{Result: v}, nil
}
func (s *Service) QueryGpuRange(ctx context.Context, r *api.QueryGpuRangeRequest) (*api.QueryGpuRangeResponse, error) {
	if r == nil || r.Start == nil || r.End == nil || r.Start.CheckValid() != nil || r.End.CheckValid() != nil {
		return nil, rpcError(biz.Fail("GPU_INVALID_QUERY", errors.New("valid range required")))
	}
	v, e := s.execute(ctx, biz.Query{Target: target(r.Target), Codes: r.MetricCodes, Start: r.Start.AsTime(), End: r.End.AsTime()})
	if e != nil {
		return nil, e
	}
	return &api.QueryGpuRangeResponse{Result: v}, nil
}
func (s *Service) execute(ctx context.Context, q biz.Query) (*api.QueryGpuResponse, error) {
	result, err := s.usecase.Execute(ctx, q)
	if err != nil {
		return nil, rpcError(err)
	}
	out := &api.QueryGpuResponse{QueryTime: ts(result.QueryTime), Start: ts(result.Start), End: ts(result.End), EffectiveStepSeconds: uint32(result.Step / time.Second), IndexComplete: result.Target.IndexComplete, IncludedResourceKinds: result.Target.IncludedKinds, UncoveredResourceKinds: result.Target.UncoveredKinds, ScopeWatermark: result.Target.Watermark}
	for _, v := range result.Series {
		row := &api.GpuSeries{MetricCode: v.Metric.Code, Unit: v.Metric.Unit, Aggregation: v.Metric.Aggregation, BaseResolutionSeconds: uint32(v.Metric.Base / time.Second), Mean: v.Mean, Peak: v.Peak, Quality: quality(v.Quality), MetricAvailableFrom: ts(v.AvailableFrom), OwnershipAvailableFrom: ts(v.OwnershipFrom), DataCutoff: ts(v.Cutoff), Capability: "UNKNOWN", EventCode: v.EventCode, Semantics: v.Semantics}
		if v.Quality.Observed > 0 {
			row.Capability = "SUPPORTED"
		}
		for _, processStart := range v.ProcessStarts {
			row.ExporterProcessStarts = append(row.ExporterProcessStarts, ts(processStart))
		}
		for _, p := range v.Points {
			row.Points = append(row.Points, &api.GpuPoint{Timestamp: ts(p.Timestamp), End: ts(p.End), Value: p.Value, Peak: p.Peak, KnownValue: p.KnownValue, KnownPeak: p.KnownPeak, Quality: quality(p.Quality), SampleTimeMin: ts(p.SampleMin), SampleTimeMax: ts(p.SampleMax)})
		}
		out.Series = append(out.Series, row)
	}
	if proto.Size(out) > 8*1024*1024 {
		return nil, rpcError(biz.Fail("GPU_BUDGET_EXCEEDED", errors.New("final GPU response exceeds 8 MiB")))
	}
	return out, nil
}
func quality(q biz.Quality) *api.GpuQuality {
	return &api.GpuQuality{Status: q.Status, Reasons: q.Reasons, CompletePoints: q.Complete, ObservedPoints: q.Observed, ExpectedPoints: q.Expected, PartialPoints: q.Partial, EmptyPoints: q.Empty, Coverage: q.Coverage, ObservedSources: q.ObservedSources, ExpectedSources: q.ExpectedSources}
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
func target(t *api.GpuTarget) biz.Target {
	if t == nil {
		return biz.Target{}
	}
	out := biz.Target{Kind: t.Kind, TenantID: t.TenantId, ResourceKind: t.ResourceKind, ResourceID: t.ResourceId, IndexComplete: t.IndexComplete, HistoryAvailableFrom: at(t.HistoryAvailableFrom), IncludedKinds: t.IncludedResourceKinds, UncoveredKinds: t.UncoveredResourceKinds, Watermark: t.ScopeWatermark}
	for _, s := range t.Scopes {
		if s == nil {
			continue
		}
		v := biz.Scope{ClusterID: s.ClusterId, NodeID: s.NodeId, NodeUID: s.NodeUid, GPUUUID: s.GpuUuid, Namespace: s.Namespace, PodName: s.PodName, ContainerName: s.ContainerName, VDeviceID: s.VdeviceId, ValidFrom: at(s.ValidFrom), ValidTo: at(s.ValidTo), EvidenceAt: at(s.EvidenceAt), OwnershipState: s.OwnershipState, ResourceKind: s.ResourceKind, DeviceID: s.DeviceId, CandidateComplete: s.CandidateComplete, Reasons: s.Reasons}
		if s.UsageRef != nil {
			v.Usage = biz.UsageRef{TenantID: s.UsageRef.TenantId, OwnerService: s.UsageRef.OwnerService, ResourceID: s.UsageRef.ResourceId, CreateOperationID: s.UsageRef.CreateOperationId}
		}
		out.Scopes = append(out.Scopes, v)
	}
	return out
}
func rpcError(err error) error {
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "GPU_QUERY_CANCELED")
	}
	reason := "GPU_QUERY_FAILED"
	var f *biz.Failure
	if errors.As(err, &f) {
		reason = f.Reason
	}
	code := map[string]codes.Code{"GPU_INVALID_QUERY": codes.InvalidArgument, "GPU_BUDGET_EXCEEDED": codes.ResourceExhausted, "GPU_CONCURRENCY_LIMIT": codes.ResourceExhausted, "GPU_SOURCE_NOT_CONFIGURED": codes.Unavailable, "GPU_SOURCE_UNAVAILABLE": codes.Unavailable, "GPU_SOURCE_RESPONSE_INVALID": codes.DataLoss, "GPU_QUERY_TIMEOUT": codes.DeadlineExceeded, "GPU_QUERY_FAILED": codes.Internal}[reason]
	if errors.Is(err, context.DeadlineExceeded) {
		code = codes.DeadlineExceeded
		reason = "GPU_QUERY_TIMEOUT"
	}
	if code == codes.OK {
		code = codes.Internal
	}
	st := status.New(code, reason)
	with, e := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: "ani-observability-service"})
	if e != nil {
		return st.Err()
	}
	return with.Err()
}
