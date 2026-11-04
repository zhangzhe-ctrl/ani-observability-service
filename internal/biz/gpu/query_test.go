package gpu

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

var epoch = time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

func fixture() Query {
	return Query{Target: Target{Kind: "TENANT", TenantID: "7ef3ded1-a63f-4ad8-8fe1-cf987a562534"}, Codes: []string{"gpu.resource.memory_used_bytes"}, Start: epoch, End: epoch.Add(time.Minute), Step: 30 * time.Second}
}
func scope(pod, res string) Scope {
	resourceID := "10000000-0000-0000-0000-000000000001"
	if res == "r2" {
		resourceID = "10000000-0000-0000-0000-000000000002"
	}
	podUID := "30000000-0000-0000-0000-000000000001"
	if pod == "p2" {
		podUID = "30000000-0000-0000-0000-000000000002"
	}
	return Scope{ClusterID: "f39bf6e2-f3ed-4191-ada3-caeebf86c220", NodeID: "node1", NodeUID: "node-uid", GPUUUID: "abc", Namespace: "ns", PodName: pod, PodUID: podUID, ContainerName: "main", VDeviceID: "0", ResourceKind: "inference", Usage: UsageRef{"7ef3ded1-a63f-4ad8-8fe1-cf987a562534", "ani-inference", resourceID, "20000000-0000-0000-0000-000000000001"}}
}
func physicalScope() Scope {
	return Scope{ClusterID: "f39bf6e2-f3ed-4191-ada3-caeebf86c220", NodeID: "node1", NodeUID: "node-uid", GPUUUID: "GPU-ABC"}
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
	s, err := Aggregate(q, m, []Source{source(scope("p1", "r1"), 100, 0), source(scope("p2", "r2"), 0, 100)})
	if err != nil {
		t.Fatal(err)
	}
	if s.Peak == nil || *s.Peak != 100 || s.Mean == nil || *s.Mean != 100 || s.Quality.Complete != 2 || s.Quality.ExpectedSources != nil {
		t.Fatalf("returned sample aggregation: %+v", s)
	}
}
func TestWideBucketKeepsPeakAndBaseGrid(t *testing.T) {
	q := fixture()
	q.End = epoch.Add(15 * time.Minute)
	q.Step = 15 * time.Minute
	m, _ := Lookup(q.Codes[0])
	v := make([]float64, 30)
	v[7] = 900
	s, err := Aggregate(q, m, []Source{source(scope("p1", "r1"), v...)})
	if err != nil || s.Peak == nil || *s.Peak != 900 || s.Mean == nil || *s.Mean != 30 || s.Points[0].Peak == nil || *s.Points[0].Peak != 900 {
		t.Fatal(s, err)
	}
}
func TestKnownPartialNeverEntersCompleteSummary(t *testing.T) {
	q := fixture()
	q.Step = time.Minute
	m, _ := Lookup(q.Codes[0])
	s, err := Aggregate(q, m, []Source{source(scope("p1", "r1"), 5)})
	if err != nil || s.Mean != nil || s.Peak != nil || s.Points[0].Value != nil || s.Points[0].KnownValue == nil || *s.Points[0].KnownValue != 5 || s.Quality.Status != "PARTIAL" || s.Quality.Complete != 1 || s.Quality.Empty != 1 {
		t.Fatal(s, err)
	}
}
func TestOriginalTimeFreshnessAndValidZero(t *testing.T) {
	q := fixture()
	m, _ := Lookup(q.Codes[0])
	src := source(scope("p1", "r1"), 0, 0)
	src.Samples[1].SampleTime = epoch.Add(-2 * time.Minute)
	s, err := Aggregate(q, m, []Source{src})
	if err != nil || s.Quality.Complete != 1 || s.Mean != nil || s.Points[0].Value == nil || *s.Points[0].Value != 0 || s.Points[1].Value != nil {
		t.Fatal(s, err)
	}
}
func TestCanonicalBusinessDuplicateUsesMaximum(t *testing.T) {
	q := fixture()
	m, _ := Lookup(q.Codes[0])
	a := source(scope("p1", "r1"), 10, 20)
	b := source(scope("p1", "r1"), 30, 20)
	s, err := Aggregate(q, m, []Source{a, b})
	if err != nil || s.Mean == nil || *s.Mean != 25 || s.Quality.Complete != 2 {
		t.Fatal(s, err)
	}
}
func TestResourceRequiresFullUsageIdentity(t *testing.T) {
	q := fixture()
	identity := scope("p1", "r1").Usage
	q.Target = Target{Kind: "RESOURCE", TenantID: identity.TenantID, ResourceID: identity.ResourceID, ResourceKind: "inference", UsageRef: &identity}
	next := scope("p1", "r1")
	next.Usage.CreateOperationID = "20000000-0000-0000-0000-000000000002"
	m, _ := Lookup(q.Codes[0])
	if _, err := Aggregate(q, m, []Source{source(next, 10, 20)}); err == nil {
		t.Fatal("different create operation must not enter a resource query")
	}
	q.Target.UsageRef = nil
	if err := q.Validate(epoch.Add(time.Hour)); err == nil {
		t.Fatal("resource query must carry the complete usage reference")
	}
}
func TestNoGridOrSamplesNeverFabricatesZero(t *testing.T) {
	q := fixture()
	q.Start = epoch.Add(time.Second)
	q.End = epoch.Add(2 * time.Second)
	m, _ := Lookup(q.Codes[0])
	s, err := Aggregate(q, m, nil)
	if err != nil || s.Quality.Expected != 0 || s.Quality.Coverage != nil || s.Quality.Status != "NO_DATA" {
		t.Fatal(s, err)
	}
	q.Start = epoch
	q.End = epoch.Add(time.Minute)
	s, err = Aggregate(q, m, nil)
	if err != nil || s.Mean != nil || s.Quality.Status != "NO_DATA" || s.Points[0].Value != nil || s.Points[0].KnownValue != nil {
		t.Fatal(s, err)
	}
}
func TestVdeviceSlotsRemainIndependent(t *testing.T) {
	q := fixture()
	a := scope("p1", "r1")
	b := a
	b.VDeviceID = "1"
	m, _ := Lookup(q.Codes[0])
	s, err := Aggregate(q, m, []Source{source(a, 10, 10), source(b, 20, 20)})
	if err != nil || s.Mean == nil || *s.Mean != 30 {
		t.Fatal(s, err)
	}
}
func TestPodReplacementUsesRecordedIdentityWithoutHistoricalLifetimes(t *testing.T) {
	q := fixture()
	old := scope("p1", "r1")
	replacement := old
	replacement.PodUID = "30000000-0000-0000-0000-000000000002"
	a := source(old, 10)
	b := source(replacement, 0, 20)
	b.Samples = b.Samples[1:]
	m, _ := Lookup(q.Codes[0])
	s, err := Aggregate(q, m, []Source{a, b})
	if err != nil || s.Mean == nil || *s.Mean != 15 || s.Quality.Status != "OK" || s.Quality.ObservedSources != 2 {
		t.Fatal(s, err)
	}
}
func TestThirtyDayBudgetAndStrictLegacySeparation(t *testing.T) {
	q := fixture()
	q.Start = epoch.Add(-30 * 24 * time.Hour)
	q.End = epoch
	if err := q.Validate(epoch); err != nil {
		t.Fatal(err)
	}
	if q.Step != 15*time.Minute {
		t.Fatal(q.Step)
	}
	q.Target.Scopes = []Scope{physicalScope()}
	if err := q.Validate(epoch); err == nil {
		t.Fatal("business queries must reject old scopes")
	}
	q = fixture()
	for i := 0; i < 17; i++ {
		q.ClusterIDs = append(q.ClusterIDs, fmt.Sprintf("40000000-0000-0000-0000-%012d", i+1))
	}
	if err := q.Validate(epoch.Add(time.Hour)); err == nil {
		t.Fatal("more than 16 managed clusters must fail")
	}
	q = fixture()
	q.Codes = []string{"gpu.physical.utilization_ratio"}
	if err := q.Validate(epoch.Add(time.Hour)); err == nil {
		t.Fatal("physical utilization cannot enter a tenant query")
	}
}
func TestPreRecordingGapsDoNotInventHistoryOrHideLaterSamples(t *testing.T) {
	q := fixture()
	src := source(scope("p1", "r1"), 10, 20)
	src.Samples = src.Samples[1:]
	m, _ := Lookup(q.Codes[0])
	s, err := Aggregate(q, m, []Source{src})
	if err != nil || s.Points[0].Value != nil || s.Points[1].Value == nil || *s.Points[1].Value != 20 || s.Mean != nil || !s.AvailableFrom.Equal(epoch.Add(30*time.Second)) {
		t.Fatal(s, err)
	}
}
func TestPhysicalConflictingScrapesRemainUnknown(t *testing.T) {
	q := fixture()
	q.Target = Target{Kind: "PHYSICAL", Scopes: []Scope{physicalScope()}}
	m, _ := Lookup("gpu.physical.memory_used_bytes")
	a := Source{Scope: physicalScope(), Key: SourceKey(physicalScope(), true), Samples: []Sample{{Timestamp: epoch, SampleTime: epoch, Value: 10}}}
	b := Source{Scope: physicalScope(), Key: SourceKey(physicalScope(), true), Samples: []Sample{{Timestamp: epoch, SampleTime: epoch, Value: 20}}}
	s, err := Aggregate(q, m, []Source{a, b})
	if err != nil || s.Points[0].Value != nil || s.Quality.Complete != 0 {
		t.Fatal(s, err)
	}
}
func TestXidSnapshotsPreserveFirstEventAndProcessBoundaries(t *testing.T) {
	q := fixture()
	q.Target = Target{Kind: "PHYSICAL", Scopes: []Scope{physicalScope()}}
	m, _ := Lookup("gpu.physical.xid_errors_total")
	s := q.Target.Scopes[0]
	src := Source{Key: SourceKey(s, true), Scope: s, Dimension: "79", Samples: []Sample{{Timestamp: epoch, SampleTime: epoch, Value: 1, ProcessStart: epoch.Add(-time.Hour)}, {Timestamp: epoch.Add(15 * time.Second), SampleTime: epoch.Add(15 * time.Second), Value: 1, ProcessStart: epoch.Add(10 * time.Second)}}}
	out, err := Aggregate(q, m, []Source{src})
	if err != nil || out.Mean != nil || out.Peak != nil || len(out.ProcessStarts) != 2 || out.Points[0].Peak != nil || out.Points[0].Value == nil || *out.Points[0].Value != 1 {
		t.Fatal(out, err)
	}
}
func TestThirtyDayFullBaseGridKeepsShortPeak(t *testing.T) {
	q := fixture()
	q.Start = epoch.Add(-30 * 24 * time.Hour)
	q.End = epoch
	if err := q.Validate(epoch); err != nil {
		t.Fatal(err)
	}
	m, _ := Lookup(q.Codes[0])
	s := scope("p1", "r1")
	src := Source{Key: SourceKey(s, false), Scope: s}
	for at := q.Start; at.Before(q.End); at = at.Add(30 * time.Second) {
		v := float64(1)
		if at.Equal(q.Start.Add(15*24*time.Hour + 7*30*time.Second)) {
			v = 999
		}
		src.Samples = append(src.Samples, Sample{Timestamp: at, SampleTime: at, Value: v})
	}
	out, err := Aggregate(q, m, []Source{src})
	if err != nil || len(out.Points) != 2880 || out.Quality.Expected != 86400 || out.Quality.Complete != 86400 || out.Peak == nil || *out.Peak != 999 {
		t.Fatal(len(out.Points), out.Quality, out.Peak, err)
	}
}
func TestProcessedBudgetIncludesCompanionValues(t *testing.T) {
	b := &Budget{}
	if err := b.Count(12000000); err != nil {
		t.Fatal(err)
	}
	if err := b.Count(1); err == nil {
		t.Fatal("budget must reject an additional timestamp or quality value")
	}
}
func TestRecentRecordingSampleDoesNotNeedHistoricalOwnershipTime(t *testing.T) {
	q := fixture()
	src := source(scope("p1", "r1"), 5, 7)
	src.Samples[0].SampleTime = epoch.Add(-time.Second)
	m, _ := Lookup(q.Codes[0])
	s, err := Aggregate(q, m, []Source{src})
	if err != nil || s.Points[0].Value == nil || s.Points[1].Value == nil || s.Mean == nil || *s.Mean != 6 {
		t.Fatal(s, err)
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
	q.Start = epoch.Add(7 * time.Second)
	q.End = q.Start.Add(7*24*time.Hour + 45*time.Second)
	p := &denseGridProvider{scope: scope("p1", "r1"), origin: q.Start}
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
	q.Target = Target{Kind: "PHYSICAL", Scopes: []Scope{physicalScope()}}
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
