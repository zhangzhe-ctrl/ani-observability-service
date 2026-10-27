// Package gpu implements the dedicated, bounded GPU query contract.
package gpu

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"
)

type UsageRef struct{ TenantID, OwnerService, ResourceID, CreateOperationID string }
type Scope struct {
	ClusterID, NodeID, NodeUID, GPUUUID, Namespace, PodName, ContainerName, VDeviceID string
	Usage                                                                             UsageRef
	ValidFrom, ValidTo, EvidenceAt                                                    time.Time
	OwnershipState, ResourceKind, DeviceID                                            string
	CandidateComplete                                                                 bool
	Reasons                                                                           []string
}
type Target struct {
	Kind, TenantID, ResourceKind, ResourceID, Watermark string
	Scopes                                              []Scope
	IndexComplete                                       bool
	HistoryAvailableFrom                                time.Time
	IncludedKinds, UncoveredKinds                       []string
}
type Query struct {
	Target     Target
	Codes      []string
	Start, End time.Time
	Latest     bool
	Step       time.Duration
	Budget     *Budget
}
type Sample struct {
	Timestamp, SampleTime time.Time
	Value                 float64
	ProcessStart          time.Time
}
type Source struct {
	Key, Dimension string
	Scope          Scope
	Samples        []Sample
}
type Metric struct {
	Code, Record, Unit, Profile, Aggregation string
	Base, Period                             time.Duration
	Nonnegative, Ratio                       bool
	Snapshot, Event                          bool
}
type Quality struct {
	Status                                                        string
	Reasons                                                       []string
	Complete, Observed, Expected, Partial, Empty, ObservedSources uint32
	ExpectedSources                                               *uint32
	Coverage                                                      *float64
}
type Point struct {
	Timestamp, End, SampleMin, SampleMax time.Time
	Value, Peak, KnownValue, KnownPeak   *float64
	Quality                              Quality
}
type Series struct {
	Metric                               Metric
	Points                               []Point
	Mean, Peak                           *float64
	Quality                              Quality
	AvailableFrom, OwnershipFrom, Cutoff time.Time
	EventCode, Semantics                 string
	ProcessStarts                        []time.Time
}
type Result struct {
	QueryTime, Start, End time.Time
	Step                  time.Duration
	Target                Target
	Series                []Series
}
type Provider interface {
	QueryGPU(context.Context, Query, Metric) ([]Source, error)
}
type Budget struct {
	values  atomic.Int64
	mu      sync.Mutex
	sources map[string]bool
}

func (b *Budget) Count(n int) error {
	if b.values.Add(int64(n)) > 12000000 {
		return Fail("GPU_BUDGET_EXCEEDED", errors.New("12 million processed sample values exceeded"))
	}
	return nil
}
func (b *Budget) ObserveSource(key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sources == nil {
		b.sources = map[string]bool{}
	}
	b.sources[key] = true
	if len(b.sources) > 128 {
		return Fail("GPU_BUDGET_EXCEEDED", errors.New("128 independent source union exceeded"))
	}
	return nil
}

type Failure struct {
	Reason string
	Err    error
}

func (e *Failure) Error() string          { return e.Reason + ": " + e.Err.Error() }
func (e *Failure) Unwrap() error          { return e.Err }
func Fail(reason string, err error) error { return &Failure{reason, err} }

var catalog = map[string]Metric{}

func init() {
	for _, m := range []Metric{
		{"gpu.physical.utilization_ratio", "nvidia_smi_utilization_gpu_ratio", "ratio", "nvml", "mean", 15 * time.Second, 15 * time.Second, true, true, false, false},
		{"gpu.physical.memory_used_bytes", "nvidia_smi_memory_used_bytes", "bytes", "nvml", "sum", 15 * time.Second, 15 * time.Second, true, false, false, false},
		{"gpu.physical.memory_total_bytes", "nvidia_smi_memory_total_bytes", "bytes", "nvml", "sum", 15 * time.Second, 15 * time.Second, true, false, false, false},
		{"gpu.physical.temperature_celsius", "nvidia_smi_temperature_gpu", "celsius", "nvml", "mean", 15 * time.Second, 15 * time.Second, false, false, false, false},
		{"gpu.physical.power_instant_watts", "nvidia_smi_power_draw_instant_watts", "watts", "nvml", "sum", 15 * time.Second, 15 * time.Second, true, false, false, false},
		{"gpu.resource.memory_used_bytes", "vGPU_device_memory_usage_in_bytes", "bytes", "hami", "sum", 30 * time.Second, 30 * time.Second, true, false, false, false},
		{"gpu.resource.memory_limit_bytes", "vGPU_device_memory_limit_in_bytes", "bytes", "hami", "sum", 30 * time.Second, 30 * time.Second, true, false, false, false},
	} {
		catalog[m.Code] = m
	}
}
func init() {
	for code, record := range map[string]string{
		"gpu.physical.ecc_corrected_aggregate_total":   "nvidia_smi_ecc_errors_corrected_aggregate_total",
		"gpu.physical.ecc_uncorrected_aggregate_total": "nvidia_smi_ecc_errors_uncorrected_aggregate_total",
		"gpu.physical.remapped_rows_correctable":       "nvidia_smi_remapped_rows_correctable",
		"gpu.physical.remapped_rows_uncorrectable":     "nvidia_smi_remapped_rows_uncorrectable",
		"gpu.physical.remapped_rows_pending":           "nvidia_smi_remapped_rows_pending",
		"gpu.physical.remapped_rows_failure":           "nvidia_smi_remapped_rows_failure",
		"gpu.physical.xid_errors_total":                "nvidia_smi_xid_errors_total",
		"gpu.physical.xid_last_timestamp_seconds":      "nvidia_smi_xid_last_timestamp_seconds",
	} {
		catalog[code] = Metric{Code: code, Record: record, Unit: "count", Profile: "nvml", Aggregation: "last_observed_snapshot", Base: 15 * time.Second, Period: 15 * time.Second, Nonnegative: true, Snapshot: true, Event: strings.Contains(code, "xid_")}
		if strings.HasSuffix(code, "timestamp_seconds") {
			m := catalog[code]
			m.Unit = "unix_seconds"
			m.Aggregation = "latest_observed_event_received_time"
			catalog[code] = m
		}
	}
}
func Lookup(code string) (Metric, bool) { m, ok := catalog[code]; return m, ok }
func NormalizeUUID(s string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "gpu-")
}
func SourceKey(s Scope, physical bool) string {
	parts := []string{s.ClusterID, s.NodeID, s.NodeUID, NormalizeUUID(s.GPUUUID)}
	if !physical {
		parts = append(parts, s.Namespace, s.PodName, s.ContainerName, s.VDeviceID)
	}
	return strings.Join(parts, "\x00")
}
func UsageKey(s Scope) string {
	return strings.Join([]string{s.Usage.TenantID, s.Usage.OwnerService, s.Usage.ResourceID, s.Usage.CreateOperationID}, "\x00")
}
func (q *Query) Validate(now time.Time) error {
	invalid := func(s string) error { return Fail("GPU_INVALID_QUERY", errors.New(s)) }
	if q.Target.Kind != "PHYSICAL" && q.Target.Kind != "TENANT" && q.Target.Kind != "RESOURCE" {
		return invalid("unknown GPU target")
	}
	if q.Target.Kind != "PHYSICAL" && !target.ValidUUID(q.Target.TenantID) {
		return invalid("valid tenant required")
	}
	if q.Target.Kind == "PHYSICAL" && q.Target.TenantID != "" {
		return invalid("physical target cannot carry tenant")
	}
	if q.Target.Kind == "RESOURCE" && (q.Target.ResourceKind == "" || q.Target.ResourceID == "") {
		return invalid("resource identity required")
	}
	if q.Start.Nanosecond() != 0 || q.End.Nanosecond() != 0 || q.End.Before(q.Start) || (!q.Latest && !q.End.After(q.Start)) || q.End.Sub(q.Start) > 30*24*time.Hour || q.Start.Before(now.Add(-30*24*time.Hour-time.Minute)) || q.End.After(now) {
		return invalid("range must be within recent 30 days")
	}
	if len(q.Codes) < 1 || len(q.Codes) > 4 {
		return invalid("one to four GPU metrics required")
	}
	seenCodes := map[string]bool{}
	sources := map[string]bool{}
	for _, s := range q.Target.Scopes {
		if !target.ValidUUID(s.ClusterID) || s.NodeID == "" || s.NodeUID == "" || NormalizeUUID(s.GPUUUID) == "" {
			return invalid("trusted cluster, node and GPU identity required")
		}
		if !s.ValidTo.IsZero() && !s.ValidFrom.IsZero() && s.ValidTo.Before(s.ValidFrom) {
			return invalid("invalid ownership interval")
		}
		if q.Target.Kind != "PHYSICAL" {
			if s.Namespace == "" || s.PodName == "" || s.ContainerName == "" || s.Usage.TenantID != q.Target.TenantID || s.Usage.OwnerService == "" || s.Usage.ResourceID == "" || s.Usage.CreateOperationID == "" {
				return invalid("complete trusted resource scope required")
			}
			if q.Target.Kind == "RESOURCE" && (s.Usage.ResourceID != q.Target.ResourceID || s.ResourceKind != q.Target.ResourceKind) {
				return invalid("resource scope mismatch")
			}
		}
		sources[SourceKey(s, q.Target.Kind == "PHYSICAL")] = true
	}
	if len(sources) > 128 {
		return Fail("GPU_BUDGET_EXCEEDED", errors.New("more than 128 independent sources"))
	}
	span := q.End.Sub(q.Start)
	q.Step = 30 * time.Second
	if span > 7*24*time.Hour {
		q.Step = 15 * time.Minute
	} else if span > 24*time.Hour {
		q.Step = 5 * time.Minute
	}
	buckets := int64(1)
	if !q.Latest {
		buckets = int64((span + q.Step - 1) / q.Step)
	}
	if buckets > 3000 || buckets*int64(len(q.Codes)) > 12000 {
		return Fail("GPU_BUDGET_EXCEEDED", errors.New("display bucket budget exceeded"))
	}
	values := int64(0)
	for _, code := range q.Codes {
		m, ok := Lookup(code)
		if !ok || seenCodes[code] || (q.Target.Kind == "PHYSICAL") != (m.Profile == "nvml") {
			return invalid("unknown, duplicate or incompatible GPU metric")
		}
		seenCodes[code] = true
		n := int64(1)
		if !q.Latest {
			n = int64(span/m.Base) + 1
		}
		pairs := int64(2)
		if m.Event {
			pairs = 4
		}
		values += n * int64(len(sources)) * pairs
	}
	if values > 12000000 {
		return Fail("GPU_BUDGET_EXCEEDED", errors.New("12 million paired sample values exceeded; reduce sources, metrics or interval"))
	}
	return nil
}

type UseCase struct {
	provider Provider
	now      func() time.Time
	mu       sync.Mutex
	tenants  map[string]int
}

func New(p Provider, now func() time.Time) *UseCase {
	if now == nil {
		now = time.Now
	}
	return &UseCase{provider: p, now: now, tenants: map[string]int{}}
}
func (u *UseCase) Execute(ctx context.Context, q Query) (*Result, error) {
	now := u.now().UTC().Truncate(time.Second)
	if q.Latest {
		q.Start = now
		q.End = now
	}
	if err := q.Validate(now); err != nil {
		return nil, err
	}
	key := q.Target.TenantID
	if key == "" {
		key = "platform"
	}
	u.mu.Lock()
	if u.tenants[key] >= 2 {
		u.mu.Unlock()
		return nil, Fail("GPU_CONCURRENCY_LIMIT", errors.New("two active queries per tenant"))
	}
	u.tenants[key]++
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		u.tenants[key]--
		if u.tenants[key] == 0 {
			delete(u.tenants, key)
		}
		u.mu.Unlock()
	}()
	batch, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	q.Budget = &Budget{}
	out := &Result{QueryTime: now, Start: q.Start, End: q.End, Step: q.Step, Target: q.Target, Series: nil}
	type item struct {
		i   int
		s   []Series
		err error
	}
	results := make(chan item, len(q.Codes))
	for i, code := range q.Codes {
		m, _ := Lookup(code)
		go func(i int, m Metric) {
			src, err := u.provider.QueryGPU(batch, q, m)
			if err != nil {
				results <- item{i: i, err: err}
				return
			}
			groups := map[string][]Source{"": src}
			if m.Event {
				groups = map[string][]Source{}
				for _, v := range src {
					groups[v.Dimension] = append(groups[v.Dimension], v)
				}
				if len(groups) == 0 {
					groups[""] = nil
				}
			}
			rows := []Series{}
			var failure error
			keys := []string{}
			for k := range groups {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				row, e := Aggregate(q, m, groups[k])
				if e != nil {
					failure = e
					break
				}
				row.EventCode = k
				rows = append(rows, row)
			}
			results <- item{i, rows, failure}
		}(i, m)
	}
	ordered := make([][]Series, len(q.Codes))
	var first error
	for range q.Codes {
		r := <-results
		if r.err != nil && first == nil {
			first = r.err
			cancel()
		}
		ordered[r.i] = r.s
	}
	if first != nil {
		return nil, first
	}
	for _, rows := range ordered {
		out.Series = append(out.Series, rows...)
	}
	if len(out.Series) > 12 || len(out.Series)*int((q.End.Sub(q.Start)+q.Step-1)/q.Step) > 12000 {
		return nil, Fail("GPU_BUDGET_EXCEEDED", errors.New("event dimension curve/bucket budget exceeded"))
	}
	if err := batch.Err(); err != nil {
		return nil, Fail("GPU_QUERY_TIMEOUT", err)
	}
	return out, nil
}

func Aggregate(q Query, m Metric, sources []Source) (Series, error) {
	out := Series{Metric: m, OwnershipFrom: q.Target.HistoryAvailableFrom, Semantics: "gauge_base_grid"}
	if m.Snapshot {
		out.Semantics = "cumulative_or_state_snapshot; no interval count or gauge mean"
	}
	if m.Event {
		out.Semantics = "events observed since exporter process start; first event creates series; received event time is not hardware occurrence time"
	}
	// Group historical bindings by original source; a raw sample is counted once.
	scopes := map[string][]Scope{}
	for _, s := range q.Target.Scopes {
		k := SourceKey(s, m.Profile == "nvml")
		scopes[k] = append(scopes[k], s)
	}
	if len(scopes) > 128 {
		return out, Fail("GPU_BUDGET_EXCEEDED", errors.New("source budget exceeded"))
	}
	// Missing vdevice metadata is a selector wildcard; retain each returned vdevice
	// as an independent source, rather than collapsing replicas or virtual slots.
	for _, src := range sources {
		if _, ok := scopes[src.Key]; ok {
			continue
		}
		for key, candidates := range scopes {
			if m.Profile == "hami" && len(candidates) > 0 && candidates[0].VDeviceID == "" {
				base := src.Scope
				base.VDeviceID = ""
				if SourceKey(base, false) == key {
					scopes[src.Key] = append([]Scope(nil), candidates...)
				}
			}
		}
	}
	for key, candidates := range scopes {
		if m.Profile == "hami" && len(candidates) > 0 && candidates[0].VDeviceID == "" {
			expanded := false
			for _, src := range sources {
				base := src.Scope
				base.VDeviceID = ""
				if src.Key != key && SourceKey(base, false) == key {
					expanded = true
				}
			}
			if expanded {
				delete(scopes, key)
			}
		}
	}
	if len(scopes) > 128 {
		return out, Fail("GPU_BUDGET_EXCEEDED", errors.New("expanded source budget exceeded"))
	}
	raw := map[string]map[int64][]Sample{}
	count := 0
	processes := map[int64]bool{}
	for _, s := range sources {
		if _, ok := scopes[s.Key]; !ok {
			return out, Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("unexpected source identity"))
		}
		if raw[s.Key] == nil {
			raw[s.Key] = map[int64][]Sample{}
		}
		for _, p := range s.Samples {
			if !p.ProcessStart.IsZero() {
				processes[p.ProcessStart.Unix()] = true
			}
			count += 2
			if count > 12000000 {
				return out, Fail("GPU_BUDGET_EXCEEDED", errors.New("sample value budget exceeded"))
			}
			raw[s.Key][p.Timestamp.Unix()] = append(raw[s.Key][p.Timestamp.Unix()], p)
		}
	}
	first := q.Start
	if !q.Latest {
		epoch := first.Unix()
		base := int64(m.Base / time.Second)
		if rem := epoch % base; rem != 0 {
			first = time.Unix(epoch+base-rem, 0).UTC()
		}
	}
	buckets := 1
	if !q.Latest {
		buckets = int((q.End.Sub(q.Start) + q.Step - 1) / q.Step)
	}
	acc := make([]bucket, buckets)
	for i := range acc {
		acc[i].reasons = map[string]bool{}
		acc[i].sourcesKnown = true
	}
	total := bucket{reasons: map[string]bool{}, sourcesKnown: true}
	for t := first; q.Latest || t.Before(q.End); t = t.Add(m.Base) {
		bi := 0
		if !q.Latest {
			bi = int(t.Sub(q.Start) / q.Step)
		}
		a := &acc[bi]
		knownAt := q.Target.IndexComplete
		if !knownAt && !q.Target.HistoryAvailableFrom.IsZero() && !t.Before(q.Target.HistoryAvailableFrom) {
			knownAt = true
			for _, s := range q.Target.Scopes {
				if (!s.ValidFrom.IsZero() && t.Before(s.ValidFrom)) || (!s.ValidTo.IsZero() && !t.Before(s.ValidTo)) {
					continue
				}
				if !s.CandidateComplete {
					knownAt = false
				}
			}
		}
		g := grid{complete: knownAt, expectedKnown: knownAt, reasons: map[string]bool{}}
		if !q.Target.HistoryAvailableFrom.IsZero() && t.Before(q.Target.HistoryAvailableFrom) {
			g.complete = false
			g.reasons["HISTORY_UNAVAILABLE"] = true
		}
		if !knownAt {
			g.reasons["HISTORY_UNAVAILABLE"] = true
		}
		if len(q.Target.UncoveredKinds) > 0 {
			g.complete = false
			g.expectedKnown = false
			g.reasons["RESOURCE_TYPE_UNCOVERED"] = true
		}
		for key, candidates := range scopes {
			active := []Scope{}
			for _, s := range candidates {
				if (!s.ValidFrom.IsZero() && t.Before(s.ValidFrom)) || (!s.ValidTo.IsZero() && !t.Before(s.ValidTo)) {
					continue
				}
				active = append(active, s)
			}
			if len(active) == 0 {
				continue
			}
			g.expected++
			reason := ""
			usage := ""
			for _, s := range active {
				if m.Profile != "nvml" {
					if !s.CandidateComplete {
						reason = "OWNERSHIP_UNKNOWN"
					}
					if s.OwnershipState != "VERIFIED" {
						reason = s.OwnershipState
						if reason == "" {
							reason = "OWNERSHIP_UNKNOWN"
						}
					}
					k := UsageKey(s)
					if usage != "" && usage != k {
						reason = "OWNERSHIP_AMBIGUOUS"
					}
					usage = k
				}
			}
			// A historical cross-resource conflict remains unsafe even outside a business interval.
			if m.Profile != "nvml" {
				allUsage := ""
				for _, s := range candidates {
					k := UsageKey(s)
					if allUsage != "" && k != allUsage {
						reason = "OWNERSHIP_AMBIGUOUS"
					}
					allUsage = k
				}
			}
			points := raw[key][t.Unix()]
			if reason == "" && len(points) == 0 {
				reason = "SCRAPE_UNAVAILABLE"
			}
			var value float64
			var st time.Time
			for j, p := range points {
				if p.SampleTime.After(t) || t.Sub(p.SampleTime) > 3*m.Period {
					reason = "STALE_SAMPLE"
				}
				if m.Profile == "hami" {
					matched := false
					for _, candidate := range candidates {
						if candidate.OwnershipState != "VERIFIED" || !candidate.CandidateComplete || (!candidate.ValidFrom.IsZero() && p.SampleTime.Before(candidate.ValidFrom)) || (!candidate.ValidTo.IsZero() && !p.SampleTime.Before(candidate.ValidTo)) {
							continue
						}
						if UsageKey(candidate) == usage {
							matched = true
						}
					}
					if !matched {
						reason = "HISTORY_UNAVAILABLE"
					}
				}
				if math.IsNaN(p.Value) || math.IsInf(p.Value, 0) || (m.Nonnegative && p.Value < 0) || (m.Ratio && p.Value > 1) {
					reason = "INVALID_VALUE"
				}
				if j > 0 && (p.Value != value || !p.SampleTime.Equal(st)) {
					reason = "SOURCE_CONFLICT"
				}
				value = p.Value
				st = p.SampleTime
			}
			if reason != "" {
				g.complete = false
				g.reasons[reason] = true
				continue
			}
			if m.Event {
				for _, p := range points {
					if p.ProcessStart.IsZero() {
						g.complete = false
						g.reasons["EXPORTER_PROCESS_START_UNKNOWN"] = true
					}
				}
			}
			g.observed++
			if m.Event && m.Unit == "unix_seconds" {
				if g.observed == 1 || value > g.value {
					g.value = value
				}
			} else {
				g.value += value
			}
			g.min = minTime(g.min, st)
			g.max = maxTime(g.max, st)
		}
		if g.expected == 0 {
			g.complete = false
			g.reasons["HISTORY_UNAVAILABLE"] = true
		}
		if m.Aggregation == "mean" && g.observed > 0 {
			g.value /= float64(g.observed)
		}
		a.add(g)
		total.add(g)
		if q.Latest {
			break
		}
	}
	for i, a := range acc {
		start := q.Start.Add(time.Duration(i) * q.Step)
		end := start.Add(q.Step)
		if q.Latest {
			end = q.End
		} else if end.After(q.End) {
			end = q.End
		}
		out.Points = append(out.Points, Point{Timestamp: start, End: end, Value: a.mean(), Peak: a.peak(), KnownValue: a.knownMean(), KnownPeak: a.knownPeak(), Quality: a.quality(len(q.Target.UncoveredKinds) == 0), SampleMin: a.min, SampleMax: a.max})
	}
	if m.Snapshot {
		for i, a := range acc {
			p := &out.Points[i]
			p.Peak = nil
			p.KnownPeak = nil
			p.Value = nil
			p.KnownValue = nil
			if a.complete > 0 {
				v := a.last
				p.Value = &v
			}
			if a.observed > 0 {
				v := a.knownLast
				p.KnownValue = &v
			}
		}
	} else {
		out.Mean = total.mean()
		out.Peak = total.peak()
	}
	for sec := range processes {
		out.ProcessStarts = append(out.ProcessStarts, time.Unix(sec, 0).UTC())
	}
	sort.Slice(out.ProcessStarts, func(i, j int) bool { return out.ProcessStarts[i].Before(out.ProcessStarts[j]) })
	out.Quality = total.quality(len(q.Target.UncoveredKinds) == 0)
	out.AvailableFrom = total.min
	out.Cutoff = total.max
	return out, nil
}

type grid struct {
	complete, zero, expectedKnown bool
	expected, observed            uint32
	value                         float64
	min, max                      time.Time
	reasons                       map[string]bool
}
type bucket struct {
	expected, complete, observed, partial                 uint32
	sum, knownSum, maximum, knownMaximum, last, knownLast float64
	maxSources, expectedSources                           uint32
	min, max                                              time.Time
	reasons                                               map[string]bool
	sourcesKnown                                          bool
}

func (b *bucket) add(g grid) {
	b.expected++
	if !g.expectedKnown {
		b.sourcesKnown = false
	}
	if g.expected > b.expectedSources {
		b.expectedSources = g.expected
	}
	if g.observed > b.maxSources {
		b.maxSources = g.observed
	}
	for k := range g.reasons {
		b.reasons[k] = true
	}
	has := g.observed > 0 || g.zero
	if has {
		b.observed++
		b.knownSum += g.value
		b.knownLast = g.value
		if b.observed == 1 || g.value > b.knownMaximum {
			b.knownMaximum = g.value
		}
		b.min = minTime(b.min, g.min)
		b.max = maxTime(b.max, g.max)
	}
	if g.complete && has {
		b.complete++
		b.sum += g.value
		b.last = g.value
		if b.complete == 1 || g.value > b.maximum {
			b.maximum = g.value
		}
	} else if has {
		b.partial++
	}
}
func (b bucket) mean() *float64 {
	if b.complete == 0 {
		return nil
	}
	v := b.sum / float64(b.complete)
	return &v
}
func (b bucket) peak() *float64 {
	if b.complete == 0 {
		return nil
	}
	v := b.maximum
	return &v
}
func (b bucket) knownMean() *float64 {
	if b.observed == 0 {
		return nil
	}
	v := b.knownSum / float64(b.observed)
	return &v
}
func (b bucket) knownPeak() *float64 {
	if b.observed == 0 {
		return nil
	}
	v := b.knownMaximum
	return &v
}
func (b bucket) quality(known bool) Quality {
	q := Quality{Status: "NO_DATA", Complete: b.complete, Observed: b.observed, Expected: b.expected, Partial: b.partial, Empty: b.expected - b.observed, ObservedSources: b.maxSources}
	if b.observed > 0 {
		q.Status = "PARTIAL"
		if b.complete == b.expected {
			q.Status = "OK"
		}
	}
	if b.expected > 0 {
		v := float64(b.complete) / float64(b.expected)
		q.Coverage = &v
	}
	if known && b.sourcesKnown {
		v := b.expectedSources
		q.ExpectedSources = &v
	}
	for k := range b.reasons {
		q.Reasons = append(q.Reasons, k)
	}
	sort.Strings(q.Reasons)
	return q
}
func minTime(a, b time.Time) time.Time {
	if a.IsZero() || (!b.IsZero() && b.Before(a)) {
		return b
	}
	return a
}
func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
