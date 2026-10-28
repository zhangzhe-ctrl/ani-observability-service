// Package usage provides bounded platform statistics over trusted tenant scopes.
package usage

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"
)

type Tenant struct {
	ID             uint32
	ResourceID     string
	GPU            gpu.Target
	GPUUnavailable string
}
type Query struct {
	Start, End        time.Time
	Tenants           []Tenant
	Clusters, Metrics []string
}
type Quality struct {
	Status             string
	Reasons            []string
	Coverage           *float64
	Observed, Expected uint32
	AvailableFrom      time.Time
}
type Point struct {
	Timestamp, End     time.Time
	Value, Known, Peak *float64
	// KnownDuration preserves sparse observation weight when GPU display buckets
	// are coarsened; missing sub-buckets never acquire a zero or full duration.
	KnownDuration time.Duration
}
type Metric struct {
	Name, Unit                       string
	Mean, Peak, KnownMean, KnownPeak *float64
	PeakAt                           time.Time
	Quality                          Quality
	Points                           []Point
	ActiveTenants                    *uint64
}
type TenantResult struct {
	ID      uint32
	Metrics []Metric
}
type Result struct {
	Start, End, Cutoff time.Time
	Step               time.Duration
	Metrics            []Metric
	Tenants            []TenantResult
}

// PodPoint is a label-scoped observation, plus expected/observed container evidence.
// OwnershipComplete is separate from sample presence and must not be inferred from it.
type PodPoint struct {
	Timestamp                                           time.Time
	Value                                               *float64
	Expected, Observed, Inventory, Conflicts, Ownership *float64
}
type PodQuery struct {
	Start, End time.Time
	Step       time.Duration
	Cluster    string
	Tenants    []string
	Metrics    []string
}
type PodResult map[string]map[string][]PodPoint
type PodReader interface {
	QueryUsagePods(context.Context, PodQuery) (PodResult, error)
}
type GPUReader interface {
	Execute(context.Context, gpu.Query) (*gpu.Result, error)
}
type Failure struct {
	Reason string
	Err    error
}

func (e *Failure) Error() string       { return e.Reason + ": " + e.Err.Error() }
func (e *Failure) Unwrap() error       { return e.Err }
func Fail(reason, detail string) error { return &Failure{reason, errors.New(detail)} }

type UseCase struct {
	gpu   GPUReader
	pods  PodReader
	now   func() time.Time
	slots chan struct{}
}

func New(g GPUReader, p PodReader, now func() time.Time) *UseCase {
	if now == nil {
		now = time.Now
	}
	return &UseCase{g, p, now, make(chan struct{}, 2)}
}

func (q Query) validate(now time.Time) (time.Duration, error) {
	if q.Start.Nanosecond() != 0 || q.End.Nanosecond() != 0 || !q.End.After(q.Start) || q.End.Sub(q.Start) > 30*24*time.Hour || q.Start.Before(now.Add(-30*24*time.Hour-time.Minute)) || q.End.After(now) {
		return 0, Fail("USAGE_INVALID_QUERY", "recent 30 day range required")
	}
	if len(q.Tenants) > 128 || len(q.Clusters) < 1 || len(q.Clusters) > 16 || len(q.Metrics) < 1 || len(q.Metrics) > 3 {
		return 0, Fail("USAGE_BUDGET_EXCEEDED", "tenant, cluster or metric budget exceeded")
	}
	clusters := map[string]bool{}
	for _, c := range q.Clusters {
		if !target.ValidUUID(c) || clusters[c] {
			return 0, Fail("USAGE_INVALID_QUERY", "invalid or duplicate cluster")
		}
		clusters[c] = true
	}
	gpuRequested := false
	for _, m := range q.Metrics {
		gpuRequested = gpuRequested || m == "gpu"
	}
	ids := map[uint32]bool{}
	resources := map[string]bool{}
	sources := map[string]bool{}
	for _, t := range q.Tenants {
		if t.ID == 0 || ids[t.ID] || !target.ValidUUID(t.ResourceID) || resources[t.ResourceID] {
			return 0, Fail("USAGE_INVALID_QUERY", "invalid or duplicate tenant")
		}
		ids[t.ID] = true
		resources[t.ResourceID] = true
		if gpuRequested && t.GPUUnavailable == "" {
			if t.GPU.Kind != "TENANT" || t.GPU.TenantID != t.ResourceID {
				return 0, Fail("USAGE_INVALID_QUERY", "trusted tenant GPU target required")
			}
			for _, s := range t.GPU.Scopes {
				if !clusters[s.ClusterID] {
					return 0, Fail("USAGE_INVALID_QUERY", "GPU scope outside trusted cluster set")
				}
				sources[gpu.SourceKey(s, false)] = true
			}
		}
	}
	if len(sources) > 128 {
		return 0, Fail("USAGE_BUDGET_EXCEEDED", "128 independent GPU sources exceeded")
	}
	seen := map[string]bool{}
	for _, m := range q.Metrics {
		if m != "gpu" && m != "cpu" && m != "memory" || seen[m] {
			return 0, Fail("USAGE_INVALID_QUERY", "unknown or duplicate metric")
		}
		seen[m] = true
	}
	// Bound expensive owner/inventory joins to roughly 300-400 evaluation points.
	// This changes only the new usage statistics, not the GPU monitoring contract.
	step := 30 * time.Second
	if q.End.Sub(q.Start) > 7*24*time.Hour {
		step = 2 * time.Hour
	} else if q.End.Sub(q.Start) > 24*time.Hour {
		step = 30 * time.Minute
	} else if q.End.Sub(q.Start) > 3*time.Hour {
		step = 5 * time.Minute
	}
	n := int((q.End.Sub(q.Start) + step - 1) / step)
	if n > 3000 || n*len(q.Tenants)*len(q.Metrics) > 1200000 {
		return 0, Fail("USAGE_BUDGET_EXCEEDED", "statistics point budget exceeded")
	}
	podMetrics := len(q.Metrics)
	if gpuRequested {
		podMetrics--
	}
	values := int64(q.End.Sub(q.Start)/(30*time.Second)+1) * int64(len(sources)) * 2
	if podMetrics > 0 {
		values += int64(n) * int64(len(q.Tenants)) * int64(len(q.Clusters)) * int64(2*podMetrics+4)
	}
	if values > 12000000 {
		return 0, Fail("USAGE_BUDGET_EXCEEDED", "12 million processed sample budget exceeded")
	}
	return step, nil
}

func (u *UseCase) Execute(ctx context.Context, q Query) (*Result, error) {
	now := u.now().UTC().Truncate(time.Second)
	if q.End.After(now) && !q.End.After(now.Add(5*time.Second)) {
		q.End = now
	}
	step, err := q.validate(now)
	if err != nil {
		return nil, err
	}
	select {
	case u.slots <- struct{}{}:
		defer func() { <-u.slots }()
	default:
		return nil, Fail("USAGE_CONCURRENCY_LIMIT", "two active platform queries allowed")
	}
	batch, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	out := &Result{Start: q.Start, End: q.End, Cutoff: q.End, Step: step, Tenants: make([]TenantResult, len(q.Tenants))}
	for i, t := range q.Tenants {
		out.Tenants[i].ID = t.ID
	}
	hasGPU := false
	podMetrics := []string{}
	for _, m := range q.Metrics {
		if m == "gpu" {
			hasGPU = true
		} else {
			podMetrics = append(podMetrics, m)
		}
	}
	gpuRows := make([]Metric, len(q.Tenants))
	var gpuErr error
	if hasGPU {
		jobs := make(chan int, len(q.Tenants))
		for i := range q.Tenants {
			jobs <- i
		}
		close(jobs)
		var wg sync.WaitGroup
		var mu sync.Mutex
		for worker := 0; worker < 4; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					if batch.Err() != nil {
						return
					}
					t := q.Tenants[i]
					m, e := u.queryGPU(batch, q, t)
					if e != nil {
						mu.Lock()
						if gpuErr == nil {
							gpuErr = e
						}
						mu.Unlock()
						cancel()
						return
					}
					gpuRows[i] = m
				}
			}()
		}
		wg.Wait()
		if gpuErr != nil {
			return nil, gpuErr
		}
	}
	// One bounded grouped query per cluster, rather than one query per table row.
	podRows := map[string]PodResult{}
	clusterErrors := map[string]bool{}
	if len(podMetrics) > 0 && len(q.Tenants) > 0 {
		ids := []string{}
		for _, t := range q.Tenants {
			ids = append(ids, t.ResourceID)
		}
		jobs := make(chan string, len(q.Clusters))
		for _, c := range q.Clusters {
			jobs <- c
		}
		close(jobs)
		var wg sync.WaitGroup
		var mu sync.Mutex
		var fatal error
		for worker := 0; worker < 4; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for c := range jobs {
					var r PodResult
					var e error
					if u.pods == nil {
						e = Fail("USAGE_SOURCE_UNAVAILABLE", "pod source unavailable")
					} else {
						r, e = u.pods.QueryUsagePods(batch, PodQuery{q.Start, q.End, step, c, ids, podMetrics})
					}
					mu.Lock()
					if e != nil {
						var f *Failure
						if errors.As(e, &f) && (f.Reason == "USAGE_BUDGET_EXCEEDED" || f.Reason == "USAGE_SOURCE_RESPONSE_INVALID") {
							if fatal == nil {
								fatal = e
							}
							cancel()
						} else {
							clusterErrors[c] = true
						}
					} else {
						podRows[c] = r
					}
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if fatal != nil {
			return nil, fatal
		}
	}
	if err := batch.Err(); err != nil {
		return nil, err
	}
	for i, t := range q.Tenants {
		for _, m := range q.Metrics {
			var metric Metric
			if m == "gpu" {
				metric = projectGPU(q, gpuRows[i], step)
			} else {
				metric = podMetric(q, t, m, step, podRows, clusterErrors)
			}
			out.Tenants[i].Metrics = append(out.Tenants[i].Metrics, metric)
		}
	}
	for mi, name := range q.Metrics {
		if name == "gpu" {
			// Sum tenants on the original shared GPU grid before coarsening. This
			// retains sparse observations without adding independently timed peaks.
			native := platformMetric(q, name, gpuGrid(q), gpuRows)
			out.Metrics = append(out.Metrics, projectGPU(q, native, step))
			continue
		}
		rows := make([]Metric, len(out.Tenants))
		for i := range out.Tenants {
			rows[i] = out.Tenants[i].Metrics[mi]
		}
		out.Metrics = append(out.Metrics, platformMetric(q, name, step, rows))
	}
	// Curves are returned only once, as the platform or single-tenant detail series.
	for i := range out.Tenants {
		for j := range out.Tenants[i].Metrics {
			out.Tenants[i].Metrics[j].Points = nil
		}
	}
	return out, nil
}

func newMetric(q Query, name string, step time.Duration) Metric {
	m := Metric{Name: name, Unit: "GiB", Quality: Quality{Status: "NO_DATA"}}
	if name == "cpu" {
		m.Unit = "cores"
	}
	for t := q.Start; t.Before(q.End); t = t.Add(step) {
		end := t.Add(step)
		if end.After(q.End) {
			end = q.End
		}
		m.Points = append(m.Points, Point{Timestamp: t, End: end})
	}
	return m
}
func pointer(v float64) *float64 { return &v }
func scaled(v *float64, divisor float64) *float64 {
	if v == nil {
		return nil
	}
	return pointer(*v / divisor)
}
func (u *UseCase) queryGPU(ctx context.Context, q Query, t Tenant) (Metric, error) {
	// Formal GPU queries keep their native read grid. A coarse instant-query
	// step would discard short-lived workloads between display ticks.
	step := gpuGrid(q)
	m := newMetric(q, "gpu", step)
	if t.GPUUnavailable != "" {
		m.Quality.Status = "UNAVAILABLE"
		m.Quality.Reasons = []string{t.GPUUnavailable}
		finish(&m)
		return m, nil
	}
	// An empty complete historical inventory is the only non-sample zero proof.
	if len(t.GPU.Scopes) == 0 && t.GPU.IndexComplete && !t.GPU.HistoryAvailableFrom.IsZero() && !q.Start.Before(t.GPU.HistoryAvailableFrom) && len(t.GPU.UncoveredKinds) == 0 {
		for i := range m.Points {
			m.Points[i].Value = pointer(0)
			m.Points[i].Known = pointer(0)
		}
		finish(&m)
		return m, nil
	}
	if u.gpu == nil {
		return m, Fail("USAGE_SOURCE_UNAVAILABLE", "GPU reader unavailable")
	}
	r, e := u.gpu.Execute(ctx, gpu.Query{Target: t.GPU, Codes: []string{"gpu.resource.memory_used_bytes"}, Start: q.Start, End: q.End})
	if e != nil {
		var f *gpu.Failure
		if errors.As(e, &f) && (f.Reason == "GPU_BUDGET_EXCEEDED" || f.Reason == "GPU_CONCURRENCY_LIMIT") {
			return m, Fail("USAGE_BUDGET_EXCEEDED", "GPU budget exceeded")
		}
		if errors.As(e, &f) && f.Reason == "GPU_INVALID_QUERY" {
			return m, Fail("USAGE_INVALID_QUERY", "invalid trusted GPU target")
		}
		m.Quality.Status = "UNAVAILABLE"
		m.Quality.Reasons = []string{"GPU_SOURCE_UNAVAILABLE"}
		finish(&m)
		return m, nil
	}
	if len(r.Series) != 1 || r.Step != step || len(r.Series[0].Points) != len(m.Points) {
		return m, Fail("USAGE_SOURCE_RESPONSE_INVALID", "GPU grid mismatch")
	}
	s := r.Series[0]
	m.Quality.Reasons = append(m.Quality.Reasons, s.Quality.Reasons...)
	m.Quality.AvailableFrom = s.AvailableFrom
	if s.OwnershipFrom.After(m.Quality.AvailableFrom) {
		m.Quality.AvailableFrom = s.OwnershipFrom
	}
	for i, p := range s.Points {
		if !p.Timestamp.Equal(m.Points[i].Timestamp) {
			return m, Fail("USAGE_SOURCE_RESPONSE_INVALID", "GPU timestamp mismatch")
		}
		if p.Quality.Status == "OK" {
			m.Points[i].Value = scaled(p.Value, 1<<30)
		}
		m.Points[i].Known = scaled(p.KnownValue, 1<<30)
		if m.Points[i].Known == nil {
			m.Points[i].Known = m.Points[i].Value
		}
	}
	finish(&m)
	return m, nil
}

func gpuGrid(q Query) time.Duration {
	// These are the existing formal GPU response grids, checked against the
	// actual response in queryGPU. No existing GPU query behavior is changed.
	if q.End.Sub(q.Start) > 7*24*time.Hour {
		return 15 * time.Minute
	}
	if q.End.Sub(q.Start) > 24*time.Hour {
		return 5 * time.Minute
	}
	return 30 * time.Second
}

func projectGPU(q Query, source Metric, step time.Duration) Metric {
	m := newMetric(q, "gpu", step)
	m.Quality.Status = source.Quality.Status
	m.Quality.Reasons = append([]string{}, source.Quality.Reasons...)
	m.Quality.AvailableFrom = source.Quality.AvailableFrom
	m.ActiveTenants = source.ActiveTenants
	values, known := make([]float64, len(m.Points)), make([]float64, len(m.Points))
	completeDuration, knownDuration := make([]time.Duration, len(m.Points)), make([]time.Duration, len(m.Points))
	for _, p := range source.Points {
		i := int(p.Timestamp.Sub(q.Start) / step)
		if i < 0 || i >= len(m.Points) {
			continue
		}
		duration := p.End.Sub(p.Timestamp)
		if p.Value != nil {
			values[i] += *p.Value * duration.Seconds()
			completeDuration[i] += duration
		}
		if p.Known != nil {
			observed := p.KnownDuration
			if observed <= 0 {
				observed = duration
			}
			known[i] += *p.Known * observed.Seconds()
			knownDuration[i] += observed
		}
	}
	for i := range m.Points {
		duration := m.Points[i].End.Sub(m.Points[i].Timestamp)
		if completeDuration[i] == duration {
			m.Points[i].Value = pointer(values[i] / duration.Seconds())
		}
		if knownDuration[i] > 0 {
			m.Points[i].Known = pointer(known[i] / knownDuration[i].Seconds())
			m.Points[i].KnownDuration = knownDuration[i]
		}
	}
	finish(&m)
	return m
}

func podMetric(q Query, t Tenant, name string, step time.Duration, clusters map[string]PodResult, failures map[string]bool) Metric {
	m := newMetric(q, name, step)
	if len(failures) == len(q.Clusters) {
		m.Quality.Status = "UNAVAILABLE"
	}
	reasons := map[string]bool{}
	for i := range m.Points {
		sum := float64(0)
		observed := false
		complete := true
		for _, c := range q.Clusters {
			if failures[c] {
				reasons["CLUSTER_SOURCE_UNAVAILABLE"] = true
				complete = false
				continue
			}
			rows := clusters[c][t.ResourceID][name]
			if i >= len(rows) || !rows[i].Timestamp.Equal(m.Points[i].Timestamp) {
				reasons["SCRAPE_UNAVAILABLE"] = true
				complete = false
				continue
			}
			p := rows[i]
			if p.Value != nil {
				sum += *p.Value
				observed = true
			}
			if p.Expected == nil || p.Observed == nil || *p.Expected != *p.Observed {
				reasons["CONTAINER_COVERAGE_INCOMPLETE"] = true
				complete = false
			}
			if p.Inventory == nil || *p.Inventory <= 0 {
				reasons["POD_INVENTORY_UNAVAILABLE"] = true
				complete = false
			}
			if p.Conflicts == nil {
				reasons["OWNERSHIP_COVERAGE_UNVERIFIED"] = true
				complete = false
			} else if *p.Conflicts > 0 {
				reasons["POD_OWNERSHIP_CONFLICT"] = true
				complete = false
			}
			if p.Ownership == nil || *p.Ownership <= 0 {
				reasons["OWNERSHIP_COVERAGE_UNVERIFIED"] = true
				complete = false
			}
		}
		if observed {
			if name != "cpu" {
				sum /= 1 << 30
			}
			m.Points[i].Known = pointer(sum)
			if complete {
				m.Points[i].Value = pointer(sum)
			}
		}
	}
	for r := range reasons {
		m.Quality.Reasons = append(m.Quality.Reasons, r)
	}
	finish(&m)
	return m
}

func platformMetric(q Query, name string, step time.Duration, rows []Metric) Metric {
	m := newMetric(q, name, step)
	reasons := map[string]bool{}
	active := uint64(0)
	allComplete := true
	allUnavailable := len(rows) > 0
	for _, r := range rows {
		allUnavailable = allUnavailable && r.Quality.Status == "UNAVAILABLE"
		positive := false
		for _, p := range r.Points {
			if p.Known != nil && *p.Known > 0 {
				positive = true
			}
		}
		if positive {
			active++
		}
		if r.Quality.Status != "OK" {
			allComplete = false
		}
		for _, reason := range r.Quality.Reasons {
			reasons[reason] = true
		}
		if r.Quality.AvailableFrom.After(m.Quality.AvailableFrom) {
			m.Quality.AvailableFrom = r.Quality.AvailableFrom
		}
	}
	for i := range m.Points {
		sum, known := float64(0), float64(0)
		complete := true
		hasKnown := false
		for _, r := range rows {
			p := r.Points[i]
			if p.Value == nil {
				complete = false
			} else {
				sum += *p.Value
			}
			if p.Known != nil {
				known += *p.Known
				hasKnown = true
			}
		}
		if len(rows) == 0 {
			complete = true
			hasKnown = true
		}
		if complete {
			m.Points[i].Value = pointer(sum)
		}
		if hasKnown {
			m.Points[i].Known = pointer(known)
		}
	}
	for reason := range reasons {
		m.Quality.Reasons = append(m.Quality.Reasons, reason)
	}
	if allUnavailable {
		m.Quality.Status = "UNAVAILABLE"
	}
	finish(&m)
	if allComplete {
		m.ActiveTenants = &active
	}
	return m
}

// Statistics use the same display grid: peak is the largest simultaneous grid
// aggregate, never the sum of tenants' independent peaks. Missing points do not
// become zero, and complete means remain absent unless the whole grid is known.
func finish(m *Metric) {
	complete, observed := uint32(0), uint32(0)
	total, knownTotal, knownWeight := float64(0), float64(0), float64(0)
	fullWeight := float64(0)
	var peak, knownPeak *float64
	var peakAt time.Time
	for i := range m.Points {
		p := &m.Points[i]
		weight := p.End.Sub(p.Timestamp).Seconds()
		fullWeight += weight
		if p.Value != nil && !math.IsNaN(*p.Value) && !math.IsInf(*p.Value, 0) {
			complete++
			total += *p.Value * weight
			p.Peak = p.Value
			if peak == nil || *p.Value > *peak {
				peak = pointer(*p.Value)
			}
		}
		if p.Known != nil && !math.IsNaN(*p.Known) && !math.IsInf(*p.Known, 0) {
			observed++
			observedWeight := weight
			if p.KnownDuration > 0 {
				observedWeight = p.KnownDuration.Seconds()
			}
			knownTotal += *p.Known * observedWeight
			knownWeight += observedWeight
			if knownPeak == nil || *p.Known > *knownPeak {
				knownPeak = pointer(*p.Known)
				peakAt = p.Timestamp
			}
			if m.Quality.AvailableFrom.IsZero() {
				m.Quality.AvailableFrom = p.Timestamp
			}
		}
	}
	m.Quality.Observed = observed
	m.Quality.Expected = uint32(len(m.Points))
	coverage := float64(complete) / float64(len(m.Points))
	m.Quality.Coverage = &coverage
	if knownWeight > 0 {
		m.KnownMean = pointer(knownTotal / knownWeight)
		m.KnownPeak = knownPeak
		m.PeakAt = peakAt
	}
	if complete == uint32(len(m.Points)) && fullWeight > 0 {
		m.Mean = pointer(total / fullWeight)
		m.Peak = peak
		m.Quality.Status = "OK"
	} else if observed > 0 {
		m.Quality.Status = "PARTIAL"
	} else if m.Quality.Status != "UNAVAILABLE" {
		m.Quality.Status = "NO_DATA"
	}
	sort.Strings(m.Quality.Reasons)
	m.Quality.Reasons = unique(m.Quality.Reasons)
}
func unique(v []string) []string {
	out := v[:0]
	for _, s := range v {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
