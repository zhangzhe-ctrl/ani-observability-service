package monitoring

import (
	"context"
	"errors"
	"log/slog"
	"time"

	api "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/monitoring/v1"
	targetapi "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/target/v1"
	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/monitoring"
	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	api.UnimplementedMonitoringServiceServer
	usecase *biz.UseCase
	logger  *slog.Logger
}

func New(usecase *biz.UseCase, logger *slog.Logger) *Service {
	return &Service{usecase: usecase, logger: logger}
}

func (s *Service) QueryRange(ctx context.Context, req *api.QueryRangeRequest) (*api.QueryRangeResponse, error) {
	if req == nil || req.Start == nil || req.End == nil || req.Start.CheckValid() != nil || req.End.CheckValid() != nil {
		return nil, rpcError(biz.Fail(biz.InvalidQuery, errors.New("valid start and end are required")))
	}
	result, err := s.usecase.Execute(ctx, biz.Query{Target: scope(req.Target), Codes: req.MetricCodes, Start: req.Start.AsTime(), End: req.End.AsTime(), Step: time.Duration(req.StepSeconds) * time.Second})
	if err != nil {
		return nil, s.failure(ctx, err)
	}
	return &api.QueryRangeResponse{Start: timestamppb.New(result.Start), End: timestamppb.New(result.End), StepSeconds: uint32(result.Step / time.Second), Series: series(result.Series)}, nil
}

func (s *Service) QueryLatest(ctx context.Context, req *api.QueryLatestRequest) (*api.QueryLatestResponse, error) {
	if req == nil {
		return nil, rpcError(biz.Fail(biz.InvalidQuery, errors.New("request required")))
	}
	result, err := s.usecase.Execute(ctx, biz.Query{Target: scope(req.Target), Codes: req.MetricCodes, Latest: true})
	if err != nil {
		return nil, s.failure(ctx, err)
	}
	return &api.QueryLatestResponse{EvaluatedAt: timestamppb.New(result.EvaluatedAt), Series: series(result.Series)}, nil
}

func scope(v *targetapi.TargetScope) target.Scope {
	if v == nil {
		return target.Scope{}
	}
	return target.Scope{Kind: int32(v.Kind), TenantID: v.TenantId, InstanceID: v.InstanceId, ClusterID: v.ClusterId, Namespace: v.Namespace, ContainerName: v.ContainerName, PodUID: v.PodUid}
}

func series(input []biz.Series) []*api.MetricSeries {
	out := make([]*api.MetricSeries, 0, len(input))
	states := map[string]api.SeriesStatus{"OK": api.SeriesStatus_SERIES_STATUS_OK, "PARTIAL": api.SeriesStatus_SERIES_STATUS_PARTIAL, "NO_DATA": api.SeriesStatus_SERIES_STATUS_NO_DATA}
	gaps := map[string]api.GapReason{"NO_DATA": api.GapReason_GAP_REASON_NO_DATA, "AMBIGUOUS_SOURCE": api.GapReason_GAP_REASON_AMBIGUOUS_SOURCE, "INVALID_VALUE": api.GapReason_GAP_REASON_INVALID_VALUE}
	for _, item := range input {
		row := &api.MetricSeries{MetricCode: item.Code, Unit: item.Unit, Status: states[item.Status], Points: make([]*api.MetricPoint, 0, len(item.Points))}
		for _, point := range item.Points {
			row.Points = append(row.Points, &api.MetricPoint{Timestamp: timestamppb.New(point.Timestamp), Value: point.Value, GapReason: gaps[point.GapReason]})
		}
		out = append(out, row)
	}
	return out
}

func (s *Service) failure(ctx context.Context, err error) error {
	if s.logger != nil {
		s.logger.ErrorContext(ctx, "monitoring query failed", "error", err)
	}
	return rpcError(err)
}

func rpcError(err error) error {
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	reason := biz.QueryFailed
	var failure *biz.Failure
	if errors.As(err, &failure) {
		reason = failure.Reason
	} else if errors.Is(err, context.DeadlineExceeded) {
		reason = biz.QueryTimeout
	}
	code := map[string]codes.Code{biz.InvalidQuery: codes.InvalidArgument, biz.ResultTooLarge: codes.ResourceExhausted, biz.ConcurrencyLimit: codes.ResourceExhausted, biz.SourceNotConfigured: codes.Unavailable, biz.SourceUnavailable: codes.Unavailable, biz.SourceResponseInvalid: codes.DataLoss, biz.QueryTimeout: codes.DeadlineExceeded, biz.QueryFailed: codes.Internal}[reason]
	if code == codes.OK {
		code = codes.Internal
		reason = biz.QueryFailed
	}
	st := status.New(code, reason)
	with, detailErr := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: "ani-observability-service"})
	if detailErr != nil {
		return status.Error(codes.Internal, "MONITORING_QUERY_FAILED")
	}
	return with.Err()
}
