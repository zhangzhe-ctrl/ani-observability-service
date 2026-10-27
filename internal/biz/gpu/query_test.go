package gpu

import (
	"context"
	"reflect"
	"testing"
	"time"
)

var epoch = time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

func fixture() Query {
	return Query{Target: Target{Kind: "TENANT", TenantID: "7ef3ded1-a63f-4ad8-8fe1-cf987a562534", IndexComplete: true, Scopes: []Scope{scope("p1", "r1"), scope("p2", "r2")}}, Codes: []string{"gpu.resource.memory_used_bytes"}, Start: epoch, End: epoch.Add(time.Minute), Step: 30 * time.Second}
}
func scope(pod, res string) Scope {
	return Scope{ClusterID: "f39bf6e2-f3ed-4191-ada3-caeebf86c220", NodeID: "node1", NodeUID: "node-uid", GPUUUID: "GPU-ABC", Namespace: "ns", PodName: pod, ContainerName: "main", VDeviceID: "0", Usage: UsageRef{"7ef3ded1-a63f-4ad8-8fe1-cf987a562534", "ani-inference", res, "original-create"}, OwnershipState: "VERIFIED", CandidateComplete: true}
}
func source(s Scope, values ...float64) Source {
	o := Source{Scope: s, Key: SourceKey(s, false)}
	for i, v := range values {
		at := epoch.Add(time.Duration(i) * 30 * time.Second)
		o.Samples = append(o.Samples, Sample{Timestamp: at, SampleTime: at, Value: v})
	}
	return o
}
func TestSimultaneousSumAndWeightedSummary(t *testing.T) {
	q := fixture()
	m, _ := Lookup(q.Codes[0])
	s, err := Aggregate(q, m, []Source{source(q.Target.Scopes[0], 100, 0), source(q.Target.Scopes[1], 0, 100)})
	if err != nil {
		t.Fatal(err)
	}
	if s.Peak == nil || *s.Peak != 100 || *s.Mean != 100 {
		t.Fatalf("simultaneous sum: %+v", s)
	}
	if s.Quality.Complete != 2 {
		t.Fatal(s.Quality)
	}
}
func TestWideBucketKeepsPeakAndBaseGrid(t *testing.T) {
	q := fixture()
	q.End = epoch.Add(15 * time.Minute)
	q.Step = 15 * time.Minute
	q.Target.Scopes = q.Target.Scopes[:1]
	m, _ := Lookup(q.Codes[0])
	v := make([]float64, 30)
	v[7] = 900
	s, _ := Aggregate(q, m, []Source{source(q.Target.Scopes[0], v...)})
	if s.Peak == nil || *s.Peak != 900 || *s.Mean != 30 || *s.Points[0].Peak != 900 {
		t.Fatal(s)
	}
}
func TestKnownPartialNeverEntersCompleteSummary(t *testing.T) {
	q := fixture()
	m, _ := Lookup(q.Codes[0])
	s, _ := Aggregate(q, m, []Source{source(q.Target.Scopes[0], 5, 7)})
	if s.Mean != nil || s.Peak != nil || s.Points[0].Value != nil || s.Points[0].KnownValue == nil || *s.Points[0].KnownValue != 5 || s.Quality.Status != "PARTIAL" || s.Quality.Partial != 2 {
		t.Fatal(s)
	}
}
func TestOriginalTimeFreshnessAndValidZero(t *testing.T) {
	q := fixture()
	q.Target.Scopes = q.Target.Scopes[:1]
	m, _ := Lookup(q.Codes[0])
	src := source(q.Target.Scopes[0], 0, 0)
	src.Samples[1].SampleTime = epoch.Add(-2 * time.Minute)
	s, _ := Aggregate(q, m, []Source{src})
	if s.Quality.Complete != 1 || s.Mean == nil || *s.Mean != 0 || s.Points[1].Value != nil {
		t.Fatal(s)
	}
}
func TestBindingDuplicateAndConflictingScrape(t *testing.T) {
	q := fixture()
	q.Target.Scopes = []Scope{q.Target.Scopes[0], q.Target.Scopes[0]}
	m, _ := Lookup(q.Codes[0])
	src := source(q.Target.Scopes[0], 10, 20)
	s, _ := Aggregate(q, m, []Source{src, src})
	if *s.Mean != 15 {
		t.Fatal(s)
	}
	src.Samples[0].Value = 999
	s, _ = Aggregate(q, m, []Source{source(q.Target.Scopes[0], 10, 20), src})
	if s.Points[0].Value != nil || s.Quality.Complete != 1 {
		t.Fatal(s)
	}
}
func TestOwnershipReuseNeverSwitchesByTime(t *testing.T) {
	q := fixture()
	old := q.Target.Scopes[0]
	next := old
	next.Usage.ResourceID = "different"
	old.ValidTo = epoch
	next.ValidFrom = epoch
	q.Target.Scopes = []Scope{old, next}
	m, _ := Lookup(q.Codes[0])
	s, _ := Aggregate(q, m, []Source{source(next, 10, 20)})
	if s.Quality.Observed != 0 || s.Mean != nil {
		t.Fatal(s)
	}
}
func TestNoGridAndNoHistoryNoFabricatedZero(t *testing.T) {
	q := fixture()
	q.Start = epoch.Add(time.Second)
	q.End = epoch.Add(2 * time.Second)
	m, _ := Lookup(q.Codes[0])
	s, _ := Aggregate(q, m, nil)
	if s.Quality.Expected != 0 || s.Quality.Coverage != nil || s.Quality.Status != "NO_DATA" {
		t.Fatal(s)
	}
	q.Start = epoch
	q.End = epoch.Add(time.Minute)
	q.Target.Scopes = nil
	s, _ = Aggregate(q, m, nil)
	if s.Mean != nil || s.Quality.Status != "NO_DATA" {
		t.Fatal(s)
	}
}
func TestSourceVdeviceWildcardDoesNotCollapse(t *testing.T) {
	q := fixture()
	q.Target.Scopes = q.Target.Scopes[:1]
	q.Target.Scopes[0].VDeviceID = ""
	a := q.Target.Scopes[0]
	a.VDeviceID = "0"
	b := a
	b.VDeviceID = "1"
	m, _ := Lookup(q.Codes[0])
	s, e := Aggregate(q, m, []Source{source(a, 10, 10), source(b, 20, 20)})
	if e != nil || s.Mean == nil || *s.Mean != 30 {
		t.Fatal(s, e)
	}
}

type fakeProvider struct{ q Query }

func (p *fakeProvider) QueryGPU(context.Context, Query, Metric) ([]Source, error) { return nil, nil }
func TestThirtyDayBudgetAndStrictLegacySeparation(t *testing.T) {
	q := fixture()
	q.Start = epoch.Add(-30 * 24 * time.Hour)
	q.End = epoch
	q.Target.Scopes = q.Target.Scopes[:1]
	if e := q.Validate(epoch); e != nil {
		t.Fatal(e)
	}
	if q.Step != 15*time.Minute {
		t.Fatal(q.Step)
	}
	for i := 0; i < 128; i++ {
		s := scope("p"+string(rune(i)), "r")
		q.Target.Scopes = append(q.Target.Scopes, s)
	}
	if e := q.Validate(epoch); e == nil {
		t.Fatal("expected sample budget")
	}
	q = fixture()
	q.Codes = []string{"gpu.physical.utilization_ratio"}
	if e := q.Validate(epoch.Add(time.Hour)); e == nil {
		t.Fatal("physical utilization cannot enter tenant query")
	}
}

func TestPreIndexGapsDoNotNullPostIndexCompleteSamples(t *testing.T) {
	q := fixture()
	q.Target.Scopes = q.Target.Scopes[:1]
	q.Target.IndexComplete = false
	q.Target.HistoryAvailableFrom = epoch.Add(30 * time.Second)
	q.Target.Scopes[0].CandidateComplete = true
	m, _ := Lookup(q.Codes[0])
	s, e := Aggregate(q, m, []Source{source(q.Target.Scopes[0], 10, 20)})
	if e != nil || s.Points[0].Value != nil || s.Points[1].Value == nil || *s.Points[1].Value != 20 || s.Mean == nil || *s.Mean != 20 {
		t.Fatal(s, e)
	}
}
func TestXidSnapshotsPreserveFirstEventAndProcessBoundaries(t *testing.T) {
	q := fixture()
	q.Target.Kind = "PHYSICAL"
	q.Target.TenantID = ""
	q.Target.Scopes = q.Target.Scopes[:1]
	q.Step = 30 * time.Second
	m, _ := Lookup("gpu.physical.xid_errors_total")
	s := q.Target.Scopes[0]
	src := Source{Key: SourceKey(s, true), Scope: s, Dimension: "79", Samples: []Sample{{Timestamp: epoch, SampleTime: epoch, Value: 1, ProcessStart: epoch.Add(-time.Hour)}, {Timestamp: epoch.Add(15 * time.Second), SampleTime: epoch.Add(15 * time.Second), Value: 1, ProcessStart: epoch.Add(10 * time.Second)}}}
	out, e := Aggregate(q, m, []Source{src})
	if e != nil || out.Mean != nil || out.Peak != nil || len(out.ProcessStarts) != 2 || out.Points[0].Peak != nil || out.Points[0].Value == nil || *out.Points[0].Value != 1 {
		t.Fatal(out, e)
	}
}
func TestThirtyDayFullBaseGridKeepsShortPeak(t *testing.T) {
	q := fixture()
	q.Target.Scopes = q.Target.Scopes[:1]
	q.Start = epoch.Add(-30 * 24 * time.Hour)
	q.End = epoch
	if e := q.Validate(epoch); e != nil {
		t.Fatal(e)
	}
	m, _ := Lookup(q.Codes[0])
	src := Source{Key: SourceKey(q.Target.Scopes[0], false), Scope: q.Target.Scopes[0]}
	for at := q.Start; at.Before(q.End); at = at.Add(30 * time.Second) {
		v := float64(1)
		if at.Equal(q.Start.Add(15*24*time.Hour + 7*30*time.Second)) {
			v = 999
		}
		src.Samples = append(src.Samples, Sample{Timestamp: at, SampleTime: at, Value: v})
	}
	s, e := Aggregate(q, m, []Source{src})
	if e != nil || len(s.Points) != 2880 || s.Quality.Expected != 86400 || s.Quality.Complete != 86400 || s.Peak == nil || *s.Peak != 999 {
		t.Fatal(len(s.Points), s.Quality, s.Peak, e)
	}
}
func TestProcessedBudgetIncludesCompanionValues(t *testing.T) {
	b := &Budget{}
	if e := b.Count(12000000); e != nil {
		t.Fatal(e)
	}
	if e := b.Count(1); e == nil {
		t.Fatal("budget must reject an additional timestamp or quality value")
	}
}
func TestOwnershipIsMatchedAtOriginalSampleTime(t *testing.T) {
	q := fixture()
	q.Target.Scopes = q.Target.Scopes[:1]
	q.Target.Scopes[0].ValidFrom = epoch
	src := source(q.Target.Scopes[0], 5, 7)
	src.Samples[0].SampleTime = epoch.Add(-time.Second)
	m, _ := Lookup(q.Codes[0])
	s, e := Aggregate(q, m, []Source{src})
	if e != nil || s.Points[0].Value != nil || s.Points[1].Value == nil {
		t.Fatal(s, e)
	}
}

type denseGridProvider struct {
	scope  Scope
	origin time.Time
	calls  int
	events bool
}

func (p *denseGridProvider) QueryGPU(_ context.Context, q Query, m Metric) ([]Source, error) {
	p.calls++
	begin := q.Start.Unix()
	base := int64(m.Base / time.Second)
	if rem := begin % base; rem != 0 {
		begin += base - rem
	}
	groups := map[string]*Source{}
	for at := time.Unix(begin, 0).UTC(); at.Before(q.End); at = at.Add(m.Base) {
		code := ""
		if p.events {
			if at.Before(p.origin.Add(6 * time.Hour)) {
				continue
			}
			code = "79"
			if !at.Before(p.origin.Add(12 * time.Hour)) {
				code = "31"
			}
		}
		if groups[code] == nil {
			groups[code] = &Source{Key: SourceKey(p.scope, m.Profile == "nvml"), Scope: p.scope, Dimension: code}
		}
		v := float64(at.Unix() % 173)
		if at.Equal(p.origin.Add(6*time.Hour - 7*time.Second)) {
			v = 10000
		}
		point := Sample{Timestamp: at, SampleTime: at, Value: v}
		if p.events {
			point.ProcessStart = p.origin.Add(-time.Hour)
		}
		groups[code].Samples = append(groups[code].Samples, point)
	}
	out := []Source{}
	for _, s := range groups {
		out = append(out, *s)
	}
	return out, nil
}
func TestChunkFoldingMatchesWholeEpochGridWithUnalignedStart(t *testing.T) {
	q := fixture()
	q.Target.Scopes = q.Target.Scopes[:1]
	q.Start = epoch.Add(7 * time.Second)
	q.End = q.Start.Add(7*24*time.Hour + 45*time.Second)
	p := &denseGridProvider{scope: q.Target.Scopes[0], origin: q.Start}
	result, e := New(p, func() time.Time { return q.End }).Execute(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if p.calls != 29 {
		t.Fatalf("expected bounded29chunks, got%d", p.calls)
	}
	if e := q.Validate(q.End); e != nil {
		t.Fatal(e)
	}
	m, _ := Lookup(q.Codes[0])
	all, e := p.QueryGPU(context.Background(), q, m)
	if e != nil {
		t.Fatal(e)
	}
	whole, e := Aggregate(q, m, all)
	if e != nil {
		t.Fatal(e)
	}
	got := result.Series[0]
	if !reflect.DeepEqual(got.Points, whole.Points) || !reflect.DeepEqual(got.Quality, whole.Quality) || *got.Mean != *whole.Mean || *got.Peak != *whole.Peak {
		t.Fatalf("chunk fold changed base-grid/bucket/weighted statistics: got=%v want=%v means=%v/%v", got.Quality, whole.Quality, got.Mean, whole.Mean)
	}
}
func TestChunkFoldingPreservesLateAndDisappearingXidDimensions(t *testing.T) {
	q := fixture()
	q.Target.Kind = "PHYSICAL"
	q.Target.TenantID = ""
	q.Target.Scopes = q.Target.Scopes[:1]
	q.Codes = []string{"gpu.physical.xid_errors_total"}
	q.Start = epoch
	q.End = q.Start.Add(13 * time.Hour)
	p := &denseGridProvider{scope: q.Target.Scopes[0], origin: q.Start, events: true}
	result, e := New(p, func() time.Time { return q.End }).Execute(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if len(result.Series) != 2 {
		t.Fatal(result.Series)
	}
	if e := q.Validate(q.End); e != nil {
		t.Fatal(e)
	}
	m, _ := Lookup(q.Codes[0])
	all, e := p.QueryGPU(context.Background(), q, m)
	if e != nil {
		t.Fatal(e)
	}
	for _, got := range result.Series {
		group := []Source{}
		for _, s := range all {
			if s.Dimension == got.EventCode {
				group = append(group, s)
			}
		}
		whole, e := Aggregate(q, m, group)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got.Points, whole.Points) || !reflect.DeepEqual(got.Quality, whole.Quality) || !reflect.DeepEqual(got.ProcessStarts, whole.ProcessStarts) {
			t.Fatalf("event dimension%s changed across chunks: %v/%v", got.EventCode, got.Quality, whole.Quality)
		}
	}
}
