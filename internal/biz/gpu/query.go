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
	ClusterID, NodeID, NodeUID, GPUUUID, DeviceID string
	// Business source identity comes from recorded Prometheus labels, never from
	// a caller-supplied historical scope.
	Namespace, PodName, PodUID, ContainerName, VDeviceID, ResourceKind string
	Usage                                                              UsageRef
}
type Target struct {
	Kind, TenantID, ResourceKind, ResourceID string
	Scopes                                   []Scope
	UsageRef                                 *UsageRef
}
type Query struct {
	Target Target
	Codes  []string
	// Internal trusted cluster subset for QueryUsage. Empty means all configured
	// sources for a business query; physical queries derive it from Scopes.
	ClusterIDs []string
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
	Metric                Metric
	Points                []Point
	Mean, Peak            *float64
	Quality               Quality
	AvailableFrom, Cutoff time.Time
	EventCode, Semantics  string
	ProcessStarts         []time.Time
	ResourceKinds         []string
	sourceKeys            map[string]bool
}
type Result struct {
	QueryTime, Start, End time.Time
	Step                  time.Duration
	Target                Target
	Series                []Series
	IndexComplete         bool
	IncludedKinds         []string
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

const MaxManagedClusters = 16

func init() {
	for _, m := range []Metric{
		{"gpu.physical.utilization_ratio", "nvidia_smi_utilization_gpu_ratio", "ratio", "nvml", "mean", 15 * time.Second, 15 * time.Second, true, true, false, false},
		{"gpu.physical.memory_used_bytes", "nvidia_smi_memory_used_bytes", "bytes", "nvml", "sum", 15 * time.Second, 15 * time.Second, true, false, false, false},
		{"gpu.physical.memory_total_bytes", "nvidia_smi_memory_total_bytes", "bytes", "nvml", "sum", 15 * time.Second, 15 * time.Second, true, false, false, false},
		{"gpu.physical.temperature_celsius", "nvidia_smi_temperature_gpu", "celsius", "nvml", "mean", 15 * time.Second, 15 * time.Second, false, false, false, false},
		{"gpu.physical.power_instant_watts", "nvidia_smi_power_draw_instant_watts", "watts", "nvml", "sum", 15 * time.Second, 15 * time.Second, true, false, false, false},
		{"gpu.resource.memory_used_bytes", "ani_gpu_resource_memory_used_bytes", "bytes", "hami", "sum", 30 * time.Second, 30 * time.Second, true, false, false, false},
		{"gpu.resource.memory_limit_bytes", "ani_gpu_resource_memory_limit_bytes", "bytes", "hami", "sum", 30 * time.Second, 30 * time.Second, true, false, false, false},
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
		parts = append(parts, s.Namespace, s.PodName, s.PodUID, s.ContainerName, s.VDeviceID, UsageKey(s))
	}
	return strings.Join(parts, "\x00")
}
func UsageKey(s Scope) string {
	return strings.Join([]string{s.Usage.TenantID, s.Usage.OwnerService, s.Usage.ResourceID, s.Usage.CreateOperationID}, "\x00")
}
func ResourceKind(owner string) string {
	switch owner {
	case "ani-resource-service":
		return "container"
	case "ani-inference":
		return "inference"
	case "ani-modeldev-service":
		return "modeldev_execution"
	default:
		return ""
	}
}
func (r UsageRef) Valid() bool {
	return target.ValidUUID(r.TenantID) && target.ValidUUID(r.ResourceID) &&
		target.ValidUUID(r.CreateOperationID) && ResourceKind(r.OwnerService) != ""
}
func (t Target) MatchesUsage(ref UsageRef, kind string) bool {
	if !ref.Valid() || ref.TenantID != t.TenantID || ResourceKind(ref.OwnerService) != kind {
		return false
	}
	return t.Kind == "TENANT" || t.Kind == "RESOURCE" && t.UsageRef != nil &&
		*t.UsageRef == ref && t.ResourceKind == kind
}
func (t Target) Validate() error {
	invalid := func(s string) error { return Fail("GPU_INVALID_QUERY", errors.New(s)) }
	if t.Kind != "PHYSICAL" && t.Kind != "TENANT" && t.Kind != "RESOURCE" {
		return invalid("unknown GPU target")
	}
	if t.Kind == "PHYSICAL" {
		if t.TenantID != "" || t.UsageRef != nil || t.ResourceKind != "" || t.ResourceID != "" {
			return invalid("physical target cannot carry a business identity")
		}
		return nil
	}
	if !target.ValidUUID(t.TenantID) {
		return invalid("valid tenant required")
	}
	if len(t.Scopes) != 0 {
		return invalid("business targets cannot carry physical or historical scopes")
	}
	if t.Kind == "TENANT" {
		if t.UsageRef != nil || t.ResourceKind != "" || t.ResourceID != "" {
			return invalid("tenant target cannot carry a resource identity")
		}
		return nil
	}
	if t.UsageRef == nil || !t.UsageRef.Valid() || t.UsageRef.TenantID != t.TenantID ||
		t.UsageRef.ResourceID != t.ResourceID || ResourceKind(t.UsageRef.OwnerService) != t.ResourceKind {
		return invalid("complete matching resource usage identity required")
	}
	return nil
}
func (q *Query) Validate(now time.Time) error {
	invalid := func(s string) error { return Fail("GPU_INVALID_QUERY", errors.New(s)) }
	if err := q.Target.Validate(); err != nil {
		return err
	}
	clusters := map[string]bool{}
	for _, cluster := range q.ClusterIDs {
		if !target.ValidUUID(cluster) || clusters[cluster] {
			return invalid("invalid or duplicate cluster")
		}
		clusters[cluster] = true
	}
	if len(clusters) > MaxManagedClusters {
		return Fail("GPU_BUDGET_EXCEEDED", errors.New("more than 16 managed clusters"))
	}
	if q.Target.Kind == "PHYSICAL" && len(q.ClusterIDs) != 0 {
		return invalid("physical clusters are selected by physical scopes")
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
		clusters[s.ClusterID] = true
		sources[SourceKey(s, true)] = true
	}
	if len(clusters) > MaxManagedClusters {
		return Fail("GPU_BUDGET_EXCEEDED", errors.New("more than 16 managed clusters"))
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
	if q.Budget == nil {
		q.Budget = &Budget{}
	}
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
			rows, err := u.queryMetric(batch, q, m)
			results <- item{i, rows, err}
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
	out.IndexComplete = len(out.Series) > 0
	kinds := map[string]bool{}
	returned := map[string]bool{}
	for _, series := range out.Series {
		returned[series.Metric.Code] = true
		if series.Quality.Expected == 0 || series.Quality.Complete != series.Quality.Expected || series.Quality.Observed == 0 {
			out.IndexComplete = false
		}
		for _, kind := range series.ResourceKinds {
			kinds[kind] = true
		}
	}
	for _, code := range q.Codes {
		if !returned[code] {
			out.IndexComplete = false
		}
	}
	for kind := range kinds {
		out.IncludedKinds = append(out.IncludedKinds, kind)
	}
	sort.Strings(out.IncludedKinds)
	if len(out.Series) > 12 || len(out.Series)*int((q.End.Sub(q.Start)+q.Step-1)/q.Step) > 12000 {
		return nil, Fail("GPU_BUDGET_EXCEEDED", errors.New("event dimension curve/bucket budget exceeded"))
	}
	if err := batch.Err(); err != nil {
		return nil, Fail("GPU_QUERY_TIMEOUT", err)
	}
	return out, nil
}

// Fold each bounded time chunk immediately. Six hours is divisible by every
// display step, so chunk boundaries retain the original request bucket origin
// and epoch-aligned base grid; no complete-window source matrix is retained.
func (u *UseCase) queryMetric(ctx context.Context, q Query, m Metric) ([]Series, error) {
	if q.Latest || q.End.Sub(q.Start) <= 6*time.Hour {
		return u.metricChunk(ctx, q, m)
	}
	states := map[string]Series{}
	for start := q.Start; start.Before(q.End); start = start.Add(6 * time.Hour) {
		if err := ctx.Err(); err != nil {
			return nil, Fail("GPU_QUERY_TIMEOUT", err)
		}
		chunk := q
		chunk.Start = start
		chunk.End = start.Add(6 * time.Hour)
		if chunk.End.After(q.End) {
			chunk.End = q.End
		}
		src, err := u.provider.QueryGPU(ctx, chunk, m)
		if err != nil {
			return nil, err
		}
		groups := map[string][]Source{"": src}
		if m.Event {
			groups = map[string][]Source{}
			for _, v := range src {
				groups[v.Dimension] = append(groups[v.Dimension], v)
			}
		}
		keys := map[string]bool{}
		for k := range states {
			keys[k] = true
		}
		for k := range groups {
			keys[k] = true
		}
		if len(keys) > 12 {
			return nil, Fail("GPU_BUDGET_EXCEEDED", errors.New("event-code curve budget exceeded"))
		}
		for key := range keys {
			row, err := Aggregate(chunk, m, groups[key])
			if err != nil {
				return nil, err
			}
			row.EventCode = key
			prior, ok := states[key]
			if !ok && !start.Equal(q.Start) {
				prefix := q
				prefix.End = start
				prior, err = Aggregate(prefix, m, nil)
				if err != nil {
					return nil, err
				}
				prior.EventCode = key
				ok = true
			}
			if ok {
				row = foldSeries(prior, row)
			}
			states[key] = row
		}
	}
	if len(states) == 0 {
		row, err := Aggregate(q, m, nil)
		return []Series{row}, err
	}
	keys := []string{}
	for k := range states {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Series, 0, len(keys))
	for _, k := range keys {
		out = append(out, states[k])
	}
	return out, nil
}
func (u *UseCase) metricChunk(ctx context.Context, q Query, m Metric) ([]Series, error) {
	src, err := u.provider.QueryGPU(ctx, q, m)
	if err != nil {
		return nil, err
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
	keys := []string{}
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Series, 0, len(keys))
	for _, key := range keys {
		row, err := Aggregate(q, m, groups[key])
		if err != nil {
			return nil, err
		}
		row.EventCode = key
		out = append(out, row)
	}
	return out, nil
}
func foldSeries(a, b Series) Series {
	ca, cb := a.Quality.Complete, b.Quality.Complete
	if ca+cb > 0 && !a.Metric.Snapshot {
		sum := 0.0
		if a.Mean != nil {
			sum += *a.Mean * float64(ca)
		}
		if b.Mean != nil {
			sum += *b.Mean * float64(cb)
		}
		v := sum / float64(ca+cb)
		a.Mean = &v
	}
	if b.Peak != nil && (a.Peak == nil || *b.Peak > *a.Peak) {
		v := *b.Peak
		a.Peak = &v
	}
	a.Points = append(a.Points, b.Points...)
	a.AvailableFrom = minTime(a.AvailableFrom, b.AvailableFrom)
	a.Cutoff = maxTime(a.Cutoff, b.Cutoff)
	a.Quality.Complete += b.Quality.Complete
	a.Quality.Observed += b.Quality.Observed
	a.Quality.Expected += b.Quality.Expected
	a.Quality.Partial += b.Quality.Partial
	a.Quality.Empty += b.Quality.Empty
	if a.Metric.Profile == "hami" {
		if a.sourceKeys == nil {
			a.sourceKeys = map[string]bool{}
		}
		for key := range b.sourceKeys {
			a.sourceKeys[key] = true
		}
		a.Quality.ObservedSources = uint32(len(a.sourceKeys))
	} else if b.Quality.ObservedSources > a.Quality.ObservedSources {
		a.Quality.ObservedSources = b.Quality.ObservedSources
	}
	if a.Quality.ExpectedSources == nil || b.Quality.ExpectedSources == nil {
		a.Quality.ExpectedSources = nil
	} else if *b.Quality.ExpectedSources > *a.Quality.ExpectedSources {
		v := *b.Quality.ExpectedSources
		a.Quality.ExpectedSources = &v
	}
	reasons := map[string]bool{}
	for _, r := range a.Quality.Reasons {
		reasons[r] = true
	}
	for _, r := range b.Quality.Reasons {
		reasons[r] = true
	}
	a.Quality.Reasons = nil
	for r := range reasons {
		a.Quality.Reasons = append(a.Quality.Reasons, r)
	}
	sort.Strings(a.Quality.Reasons)
	a.Quality.Status = "NO_DATA"
	if a.Quality.Observed > 0 {
		a.Quality.Status = "PARTIAL"
		if a.Quality.Complete == a.Quality.Expected {
			a.Quality.Status = "OK"
		}
	}
	if a.Quality.Expected > 0 {
		v := float64(a.Quality.Complete) / float64(a.Quality.Expected)
		a.Quality.Coverage = &v
	}
	if a.Metric.Profile == "hami" && a.Quality.Complete != a.Quality.Expected {
		a.Mean, a.Peak = nil, nil
	}
	kinds := map[string]bool{}
	for _, kind := range a.ResourceKinds {
		kinds[kind] = true
	}
	for _, kind := range b.ResourceKinds {
		kinds[kind] = true
	}
	a.ResourceKinds = nil
	for kind := range kinds {
		a.ResourceKinds = append(a.ResourceKinds, kind)
	}
	sort.Strings(a.ResourceKinds)
	starts := map[int64]bool{}
	for _, v := range a.ProcessStarts {
		starts[v.UnixNano()] = true
	}
	for _, v := range b.ProcessStarts {
		if !starts[v.UnixNano()] {
			starts[v.UnixNano()] = true
			a.ProcessStarts = append(a.ProcessStarts, v)
		}
	}
	sort.Slice(a.ProcessStarts, func(i, j int) bool { return a.ProcessStarts[i].Before(a.ProcessStarts[j]) })
	return a
}

func Aggregate(q Query, m Metric, sources []Source) (Series, error) {
	physical := m.Profile == "nvml"
	out := Series{Metric: m, Semantics: "gauge_base_grid"}
	if !physical {
		out.Semantics = "recorded_gauge_base_grid; coverage describes returned samples, not the workload inventory"
	}
	if m.Snapshot {
		out.Semantics = "cumulative_or_state_snapshot; no interval count or gauge mean"
	}
	if m.Event {
		out.Semantics = "events observed since exporter process start; first event creates series; received event time is not hardware occurrence time"
	}
	scopes := map[string]Scope{}
	for _, s := range q.Target.Scopes {
		scopes[SourceKey(s, true)] = s
	}
	if len(scopes) > 128 {
		return out, Fail("GPU_BUDGET_EXCEEDED", errors.New("source budget exceeded"))
	}
	raw := map[string]map[int64][]Sample{}
	kinds := map[string]bool{}
	processes := map[int64]bool{}
	count := 0
	for _, s := range sources {
		if s.Key != SourceKey(s.Scope, physical) {
			return out, Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("invalid source identity"))
		}
		if physical {
			if _, ok := scopes[s.Key]; !ok {
				return out, Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("unexpected physical source identity"))
			}
		} else {
			if !q.Target.MatchesUsage(s.Scope.Usage, s.Scope.ResourceKind) {
				return out, Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("unexpected business source identity"))
			}
			scopes[s.Key] = s.Scope
			if len(s.Samples) > 0 {
				kinds[s.Scope.ResourceKind] = true
			}
		}
		if len(scopes) > 128 {
			return out, Fail("GPU_BUDGET_EXCEEDED", errors.New("source budget exceeded"))
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
		acc[i].sourcesKnown = physical
	}
	total := bucket{reasons: map[string]bool{}, sourcesKnown: physical}
	for t := first; q.Latest || t.Before(q.End); t = t.Add(m.Base) {
		bi := 0
		if !q.Latest {
			bi = int(t.Sub(q.Start) / q.Step)
		}
		g := grid{complete: true, expectedKnown: physical, reasons: map[string]bool{}}
		for key := range scopes {
			points := raw[key][t.Unix()]
			// A recorded business source may exist for only part of the window.
			// No historical inventory is inferred from another timestamp.
			if !physical && len(points) == 0 {
				continue
			}
			g.expected++
			reason := ""
			if len(points) == 0 {
				reason = "SCRAPE_UNAVAILABLE"
			}
			var value float64
			var st time.Time
			for j, p := range points {
				if p.SampleTime.After(t) || t.Sub(p.SampleTime) > 3*m.Period {
					reason = "STALE_SAMPLE"
				}
				if math.IsNaN(p.Value) || math.IsInf(p.Value, 0) || (m.Nonnegative && p.Value < 0) || (m.Ratio && p.Value > 1) {
					reason = "INVALID_VALUE"
				}
				if physical {
					if j > 0 && (p.Value != value || !p.SampleTime.Equal(st)) {
						reason = "SOURCE_CONFLICT"
					}
					value, st = p.Value, p.SampleTime
				} else if j == 0 || p.Value > value {
					// Recording rules already canonicalize duplicate scrapes. Do not
					// rebuild the old per-sample value/timestamp equality proof.
					value, st = p.Value, p.SampleTime
				}
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
			if !physical {
				acc[bi].observeSource(key)
				total.observeSource(key)
			}
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
		if g.observed == 0 {
			g.complete = false
			if len(g.reasons) == 0 {
				g.reasons["SCRAPE_UNAVAILABLE"] = true
			}
		}
		if m.Aggregation == "mean" && g.observed > 0 {
			g.value /= float64(g.observed)
		}
		acc[bi].add(g)
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
		point := Point{Timestamp: start, End: end, KnownValue: a.knownMean(), KnownPeak: a.knownPeak(), Quality: a.quality(physical), SampleMin: a.min, SampleMax: a.max}
		if physical || a.complete == a.expected {
			point.Value, point.Peak = a.mean(), a.peak()
		}
		out.Points = append(out.Points, point)
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
	} else if physical || total.complete == total.expected {
		out.Mean, out.Peak = total.mean(), total.peak()
	}
	for sec := range processes {
		out.ProcessStarts = append(out.ProcessStarts, time.Unix(sec, 0).UTC())
	}
	sort.Slice(out.ProcessStarts, func(i, j int) bool { return out.ProcessStarts[i].Before(out.ProcessStarts[j]) })
	for kind := range kinds {
		out.ResourceKinds = append(out.ResourceKinds, kind)
	}
	sort.Strings(out.ResourceKinds)
	out.Quality = total.quality(physical)
	out.sourceKeys = total.sourceKeys
	out.AvailableFrom = total.min
	out.Cutoff = total.max
	return out, nil
}

type grid struct {
	complete, expectedKnown bool
	expected, observed      uint32
	value                   float64
	min, max                time.Time
	reasons                 map[string]bool
}
type bucket struct {
	expected, complete, observed, partial                 uint32
	sum, knownSum, maximum, knownMaximum, last, knownLast float64
	maxSources, expectedSources                           uint32
	min, max                                              time.Time
	reasons                                               map[string]bool
	sourcesKnown                                          bool
	sourceKeys                                            map[string]bool
}

func (b *bucket) observeSource(key string) {
	if b.sourceKeys == nil {
		b.sourceKeys = map[string]bool{}
	}
	b.sourceKeys[key] = true
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
	has := g.observed > 0
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
	if !known {
		q.ObservedSources = uint32(len(b.sourceKeys))
	}
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
