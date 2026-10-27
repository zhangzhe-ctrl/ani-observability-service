package monitoring

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"
)

type providerFunc func(context.Context, Query, Metric) ([]Source, error)

func (f providerFunc) Query(ctx context.Context, q Query, m Metric) ([]Source, error) {
	return f(ctx, q, m)
}
func validQuery() Query {
	start := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	return Query{Target: target.Scope{Kind: 1, TenantID: "11111111-1111-4111-8111-111111111111", InstanceID: "22222222-2222-4222-8222-222222222222", ClusterID: "33333333-3333-4333-8333-333333333333", Namespace: "workspace-tenant", ContainerName: "workspace"}, Codes: []string{CPU, Memory}, Start: start, End: start.Add(65 * time.Second), Step: 30 * time.Second}
}
func reason(t *testing.T, err error, want string) {
	t.Helper()
	var failure *Failure
	if !errors.As(err, &failure) || failure.Reason != want {
		t.Fatalf("error=%v; want %s", err, want)
	}
}

func TestNormalizeZeroGapsAndRuntimeHistory(t *testing.T) {
	q := validQuery()
	m, _ := Lookup(CPU, target.ContainerInstance)
	series, err := Normalize(q, m, []Source{{PodUID: "old", ContainerID: "old", Samples: []Sample{{q.Start, 0}}}, {PodUID: "new", ContainerID: "new", Samples: []Sample{{q.Start.Add(60 * time.Second), 2.5}}}})
	if err != nil {
		t.Fatal(err)
	}
	if series.Status != "PARTIAL" || len(series.Points) != 3 || series.Points[0].Value == nil || *series.Points[0].Value != 0 || series.Points[0].GapReason != "" || series.Points[1].Value != nil || series.Points[1].GapReason != "NO_DATA" || *series.Points[2].Value != 2.5 {
		t.Fatalf("unexpected history: %+v", series)
	}
	if !series.Points[2].Timestamp.Equal(q.Start.Add(60 * time.Second)) {
		t.Fatal("end was aligned or interpolated")
	}
	q.End = q.Start
	empty, err := Normalize(q, m, nil)
	if err != nil || len(empty.Points) != 1 || empty.Status != "NO_DATA" || empty.Points[0].GapReason != "NO_DATA" {
		t.Fatalf("empty start=end: %+v, %v", empty, err)
	}
}

func TestNormalizeAmbiguityBeforeInvalidAndRejectsBrokenGrid(t *testing.T) {
	q := validQuery()
	m, _ := Lookup(Memory, target.ContainerInstance)
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1} {
		one := Source{PodUID: "one", ContainerID: "one", Samples: []Sample{{q.Start, value}}}
		series, err := Normalize(q, m, []Source{one})
		if err != nil || series.Points[0].GapReason != "INVALID_VALUE" || series.Points[0].Value != nil {
			t.Fatalf("invalid sample escaped: %+v %v", series, err)
		}
		two := Source{PodUID: "two", ContainerID: "two", Samples: []Sample{{q.Start, 1}}}
		series, err = Normalize(q, m, []Source{one, two})
		if err != nil || series.Points[0].GapReason != "AMBIGUOUS_SOURCE" {
			t.Fatalf("ambiguity lost: %+v %v", series, err)
		}
	}
	for _, samples := range [][]Sample{{{q.Start, 1}, {q.Start, 2}}, {{q.Start.Add(time.Second), 1}}, {{q.End.Add(time.Second), 1}}} {
		_, err := Normalize(q, m, []Source{{PodUID: "one", ContainerID: "one", Samples: samples}})
		reason(t, err, SourceResponseInvalid)
	}
}

func TestInvalidQueryNeverCallsProvider(t *testing.T) {
	mutations := map[string]func(*Query){
		"unknown": func(q *Query) { q.Codes = []string{"bad"} }, "duplicate": func(q *Query) { q.Codes = []string{CPU, CPU} }, "empty": func(q *Query) { q.Codes = nil },
		"zero step": func(q *Query) { q.Step = 0 }, "small step": func(q *Query) { q.Step = 29 * time.Second }, "nonmultiple": func(q *Query) { q.Step = 31 * time.Second }, "large step": func(q *Query) { q.Step = 3630 * time.Second },
		"reverse": func(q *Query) { q.End = q.Start.Add(-time.Second) }, "future": func(q *Query) { q.End = q.Start.Add(2 * time.Hour) }, "too long": func(q *Query) { q.Start = q.End.Add(-24*time.Hour - time.Second) }, "fractional": func(q *Query) { q.Start = q.Start.Add(time.Nanosecond) },
		"bad tenant": func(q *Query) { q.Target.TenantID = "00000000-0000-0000-0000-000000000000" }, "bad namespace": func(q *Query) { q.Target.Namespace = "bad_name" }, "sidecar": func(q *Query) { q.Target.ContainerName = "sidecar" }, "kind": func(q *Query) { q.Target.Kind = 0 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			q := validQuery()
			mutate(&q)
			u := NewUseCase(providerFunc(func(context.Context, Query, Metric) ([]Source, error) {
				t.Fatal("invalid query reached provider")
				return nil, nil
			}), func() time.Time { return validQuery().Start.Add(time.Hour) })
			_, err := u.Execute(context.Background(), q)
			reason(t, err, InvalidQuery)
		})
	}
}

func TestMetricCountLimitAndNetworkCatalog(t *testing.T) {
	for _, code := range []string{NetworkReceive, NetworkTransmit} {
		m, ok := Lookup(code, target.ContainerInstance)
		if !ok || m.Unit != "bytes_per_second" || m.Record == "" {
			t.Fatalf("network catalog entry wrong: %+v ok=%v", m, ok)
		}
	}
	three := []string{CPU, Memory, NetworkReceive}
	four := []string{CPU, Memory, NetworkReceive, NetworkTransmit}
	for _, codes := range [][]string{three, four} {
		q := validQuery()
		q.Codes = codes
		u := NewUseCase(providerFunc(func(_ context.Context, _ Query, _ Metric) ([]Source, error) {
			return nil, nil
		}), func() time.Time { return q.End })
		result, err := u.Execute(context.Background(), q)
		if err != nil {
			t.Fatalf("%d codes rejected: %v", len(codes), err)
		}
		if len(result.Series) != len(codes) {
			t.Fatalf("series count %d != %d", len(result.Series), len(codes))
		}
		for i, code := range codes {
			if result.Series[i].Code != code {
				t.Fatalf("series[%d].Code=%s want %s", i, result.Series[i].Code, code)
			}
		}
	}
	q := validQuery()
	q.Codes = []string{CPU, Memory, NetworkReceive, NetworkTransmit, CPU}
	u := NewUseCase(providerFunc(func(context.Context, Query, Metric) ([]Source, error) {
		t.Fatal("five-code query reached provider")
		return nil, nil
	}), func() time.Time { return q.End })
	_, err := u.Execute(context.Background(), q)
	reason(t, err, InvalidQuery)
}

func trainingQuery() Query {
	q := validQuery()
	q.Target = target.Scope{Kind: target.TrainingExecution, TenantID: "11111111-1111-4111-8111-111111111111",
		InstanceID: "22222222-2222-4222-8222-222222222222", ClusterID: "33333333-3333-4333-8333-333333333333",
		Namespace: "ani-modeldev-tenant-a-v21", ContainerName: "node", PodUID: "44444444-4444-4444-8444-444444444444"}
	return q
}

func TestTrainingCatalogAndKindIsolation(t *testing.T) {
	instanceCPU, ok := Lookup(CPU, target.ContainerInstance)
	if !ok || instanceCPU.Record != "ani_instance_cpu_usage_cores" {
		t.Fatalf("instance cpu catalog wrong: %+v ok=%v", instanceCPU, ok)
	}
	trainingCPU, ok := Lookup(CPU, target.TrainingExecution)
	if !ok || trainingCPU.Record != "ani_training_cpu_usage_cores" || trainingCPU.Unit != "cores" {
		t.Fatalf("training cpu catalog wrong: %+v ok=%v", trainingCPU, ok)
	}
	trainingMemory, ok := Lookup(Memory, target.TrainingExecution)
	if !ok || trainingMemory.Record != "ani_training_memory_working_set_bytes" || trainingMemory.Unit != "bytes" {
		t.Fatalf("training memory catalog wrong: %+v ok=%v", trainingMemory, ok)
	}
	for _, code := range []string{NetworkReceive, NetworkTransmit} {
		if _, ok := Lookup(code, target.TrainingExecution); ok {
			t.Fatalf("training kind must not register %s", code)
		}
	}
	if _, ok := Lookup(CPU, 99); ok {
		t.Fatal("unknown kind resolved a metric")
	}
	q := trainingQuery()
	q.Codes = []string{NetworkReceive}
	u := NewUseCase(providerFunc(func(context.Context, Query, Metric) ([]Source, error) {
		t.Fatal("training network query reached provider")
		return nil, nil
	}), func() time.Time { return q.End })
	_, err := u.Execute(context.Background(), q)
	reason(t, err, InvalidQuery)
}

func TestLatestCapturesOneClockAndKeepsOrder(t *testing.T) {
	q := validQuery()
	q.Latest = true
	q.Codes = []string{Memory, CPU}
	var clockCalls atomic.Int32
	var mu sync.Mutex
	var observed []time.Time
	now := q.Start.Add(999 * time.Millisecond)
	u := NewUseCase(providerFunc(func(_ context.Context, received Query, _ Metric) ([]Source, error) {
		mu.Lock()
		observed = append(observed, received.Start)
		mu.Unlock()
		return nil, nil
	}), func() time.Time { clockCalls.Add(1); return now })
	result, err := u.Execute(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if clockCalls.Load() != 1 || len(observed) != 2 || !observed[0].Equal(q.Start) || !observed[1].Equal(q.Start) || !result.EvaluatedAt.Equal(q.Start) {
		t.Fatalf("batch clock differs: %+v", observed)
	}
	if result.Series[0].Code != Memory || result.Series[1].Code != CPU || len(result.Series[0].Points) != 1 {
		t.Fatalf("latest order/count: %+v", result)
	}
}

func TestBatchFailureCancelsSibling(t *testing.T) {
	q := validQuery()
	started := make(chan struct{})
	canceled := make(chan struct{})
	u := NewUseCase(providerFunc(func(ctx context.Context, _ Query, m Metric) ([]Source, error) {
		if m.Code == Memory {
			close(started)
			<-ctx.Done()
			close(canceled)
			return nil, ctx.Err()
		}
		<-started
		return nil, Fail(SourceUnavailable, errors.New("controlled external boundary failure"))
	}), func() time.Time { return q.End })
	_, err := u.Execute(context.Background(), q)
	reason(t, err, SourceUnavailable)
	select {
	case <-canceled:
	default:
		t.Fatal("sibling query survived failed batch")
	}
}

func TestRequestConcurrencyAndCancellationRelease(t *testing.T) {
	q := validQuery()
	q.Codes = []string{CPU}
	entered := make(chan struct{}, 20)
	release := make(chan struct{})
	u := NewUseCase(providerFunc(func(ctx context.Context, _ Query, _ Metric) ([]Source, error) {
		entered <- struct{}{}
		select {
		case <-release:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}), func() time.Time { return q.End })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 16)
	for range 4 {
		go func() { _, err := u.Execute(ctx, q); finished <- err }()
		<-entered
	}
	_, err := u.Execute(ctx, q)
	reason(t, err, ConcurrencyLimit)
	for i := 0; i < 12; i++ {
		other := q
		other.Target.TenantID = "44444444-4444-4444-8444-444444444444"
		if i >= 4 {
			other.Target.TenantID = "55555555-5555-4555-8555-555555555555"
		}
		if i >= 8 {
			other.Target.TenantID = "66666666-6666-4666-8666-666666666666"
		}
		go func() { _, err := u.Execute(ctx, other); finished <- err }()
		<-entered
	}
	other := q
	other.Target.TenantID = "77777777-7777-4777-8777-777777777777"
	_, err = u.Execute(ctx, other)
	reason(t, err, ConcurrencyLimit)
	cancel()
	for range 16 {
		if err := <-finished; !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled request: %v", err)
		}
	}
	close(release)
	if _, err := u.Execute(context.Background(), q); err != nil {
		t.Fatalf("tokens leaked after cancellation: %v", err)
	}
}

func TestShorterParentDeadlineIsHonored(t *testing.T) {
	q := validQuery()
	q.Codes = []string{CPU}
	u := NewUseCase(providerFunc(func(ctx context.Context, _ Query, _ Metric) ([]Source, error) { <-ctx.Done(); return nil, ctx.Err() }), func() time.Time { return q.End })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := u.Execute(ctx, q)
	reason(t, err, QueryTimeout)
}
