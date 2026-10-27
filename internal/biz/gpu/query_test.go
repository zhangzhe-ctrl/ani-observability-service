package gpu

import (
	"context"
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
