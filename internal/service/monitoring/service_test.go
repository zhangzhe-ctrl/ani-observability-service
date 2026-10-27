package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	api "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/monitoring/v1"
	targetapi "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/target/v1"
	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/monitoring"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type providerFunc func(context.Context, biz.Query, biz.Metric) ([]biz.Source, error)

func (f providerFunc) Query(ctx context.Context, q biz.Query, m biz.Metric) ([]biz.Source, error) {
	return f(ctx, q, m)
}
func rpcTarget() *targetapi.TargetScope {
	return &targetapi.TargetScope{Kind: targetapi.TargetKind_TARGET_KIND_CONTAINER_INSTANCE, TenantId: "11111111-1111-4111-8111-111111111111", InstanceId: "22222222-2222-4222-8222-222222222222", ClusterId: "33333333-3333-4333-8333-333333333333", Namespace: "workspace-a", ContainerName: "workspace"}
}

func TestScopeMapsTrainingPodUID(t *testing.T) {
	got := scope(&targetapi.TargetScope{Kind: targetapi.TargetKind_TARGET_KIND_TRAINING_EXECUTION,
		TenantId: "11111111-1111-4111-8111-111111111111", InstanceId: "22222222-2222-4222-8222-222222222222",
		ClusterId: "33333333-3333-4333-8333-333333333333", Namespace: "ani-modeldev-tenant-a-v21",
		ContainerName: "node", PodUid: "44444444-4444-4444-8444-444444444444"})
	if got.Kind != int32(targetapi.TargetKind_TARGET_KIND_TRAINING_EXECUTION) ||
		got.PodUID != "44444444-4444-4444-8444-444444444444" || got.ContainerName != "node" {
		t.Fatalf("training scope mapping lost pod uid: %+v", got)
	}
	if instance := scope(rpcTarget()); instance.PodUID != "" {
		t.Fatalf("instance scope must leave pod uid empty: %+v", instance)
	}
}

func TestRPCPreservesMeasuredZeroVersusMissing(t *testing.T) {
	start := time.Unix(1700000000, 0).UTC()
	provider := providerFunc(func(_ context.Context, q biz.Query, m biz.Metric) ([]biz.Source, error) {
		return []biz.Source{{PodUID: "one", ContainerID: "one", Samples: []biz.Sample{{Timestamp: q.Start, Value: 0}}}}, nil
	})
	s := New(biz.NewUseCase(provider, func() time.Time { return start.Add(time.Hour) }), nil)
	response, err := s.QueryRange(context.Background(), &api.QueryRangeRequest{Target: rpcTarget(), MetricCodes: []string{biz.CPU}, Start: timestamppb.New(start), End: timestamppb.New(start.Add(60 * time.Second)), StepSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := proto.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded api.QueryRangeResponse
	if err := proto.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	points := decoded.Series[0].Points
	if len(points) != 3 || points[0].Value == nil || *points[0].Value != 0 || points[0].GapReason != api.GapReason_GAP_REASON_UNSPECIFIED || points[1].Value != nil || points[1].GapReason != api.GapReason_GAP_REASON_NO_DATA || decoded.Series[0].Status != api.SeriesStatus_SERIES_STATUS_PARTIAL {
		t.Fatalf("presence/state lost: %+v", decoded.Series)
	}
}

func TestRPCMachineReasonsRemainDistinct(t *testing.T) {
	for reason, want := range map[string]codes.Code{biz.InvalidQuery: codes.InvalidArgument, biz.ResultTooLarge: codes.ResourceExhausted, biz.ConcurrencyLimit: codes.ResourceExhausted, biz.SourceNotConfigured: codes.Unavailable, biz.SourceUnavailable: codes.Unavailable, biz.SourceResponseInvalid: codes.DataLoss, biz.QueryTimeout: codes.DeadlineExceeded, biz.QueryFailed: codes.Internal} {
		t.Run(reason, func(t *testing.T) {
			err := rpcError(biz.Fail(reason, errors.New("private datasource address and query")))
			st := status.Convert(err)
			if st.Code() != want || st.Message() != reason {
				t.Fatalf("wrong public error: %v", st)
			}
			details := st.Details()
			if len(details) != 1 {
				t.Fatal("missing ErrorInfo")
			}
			info, ok := details[0].(*errdetails.ErrorInfo)
			if !ok || info.Reason != reason {
				t.Fatalf("wrong machine reason: %+v", details)
			}
		})
	}
}

func TestRPCRejectsNilAndInvalidTimestamp(t *testing.T) {
	s := New(nil, nil)
	for _, request := range []*api.QueryRangeRequest{nil, {}, {Start: &timestamppb.Timestamp{Seconds: 253402300800}, End: timestamppb.Now()}} {
		_, err := s.QueryRange(context.Background(), request)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("nil/invalid request: %v", err)
		}
	}
	if _, err := s.QueryLatest(context.Background(), nil); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("nil latest: %v", err)
	}
}
