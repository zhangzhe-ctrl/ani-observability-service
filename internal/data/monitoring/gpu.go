package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	gpu "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
)

// GPU has an independent execution semaphore and response limit. Legacy limits
// and source label validation remain unchanged.
var gpuSlots = make(chan struct{}, 16)

const gpuMaxBody = 16 * 1024 * 1024

type gpuWireSample struct {
	at    time.Time
	value float64
}

func (p *Prometheus) QueryGPU(ctx context.Context, q gpu.Query, m gpu.Metric) ([]gpu.Source, error) {
	clusters := map[string][]gpu.Scope{}
	for _, s := range q.Target.Scopes {
		clusters[s.ClusterID] = append(clusters[s.ClusterID], s)
	}
	all := []gpu.Source{}
	processed := 0
	unique := map[string]bool{}
	clusterKeys := []string{}
	for k := range clusters {
		clusterKeys = append(clusterKeys, k)
	}
	sort.Strings(clusterKeys)
	for _, cluster := range clusterKeys {
		s := p.sources[cluster]
		if s == nil {
			return nil, gpu.Fail("GPU_SOURCE_NOT_CONFIGURED", errors.New("no configured datasource"))
		}
		expr := gpuExpression(clusters[cluster], m)
		first := q.Start
		if !q.Latest {
			base := int64(m.Base / time.Second)
			if r := first.Unix() % base; r != 0 {
				first = time.Unix(first.Unix()+base-r, 0).UTC()
			}
		}
		for start := first; q.Latest || start.Before(q.End); {
			end := start
			if !q.Latest {
				end = start.Add(6 * time.Hour)
				if end.After(q.End) {
					end = q.End
				}
				end = end.Add(-time.Nanosecond)
			}
			values, err := s.gpuRequest(ctx, q, m, expr, start, end)
			if err != nil {
				return nil, err
			}
			stamps, err := s.gpuRequest(ctx, q, m, "timestamp("+expr+")", start, end)
			if err != nil {
				return nil, err
			}
			processStarts := map[string]map[int64]time.Time{}
			processSampleTimes := map[string]map[int64]float64{}
			if m.Event {
				processWire, e := s.gpuRequest(ctx, q, m, gpuProcessExpression(clusters[cluster]), start, end)
				if e != nil {
					return nil, e
				}
				processTimes, e := s.gpuRequest(ctx, q, m, "timestamp("+gpuProcessExpression(clusters[cluster])+")", start, end)
				if e != nil {
					return nil, e
				}
				for _, w := range processTimes {
					points, e := gpuPoints(w, q.Latest, start, end, m.Base)
					if e != nil {
						return nil, e
					}
					if q.Budget != nil {
						if e := q.Budget.Count(len(points)); e != nil {
							return nil, e
						}
					}
					key := gpuProcessPath(w.Metric)
					if processSampleTimes[key] == nil {
						processSampleTimes[key] = map[int64]float64{}
					}
					for _, pt := range points {
						processSampleTimes[key][pt.at.Unix()] = pt.value
					}
				}
				for _, w := range processWire {
					if w.Metric["__name__"] != "process_start_time_seconds" || w.Metric["ani_cluster_id"] != cluster || w.Metric["ani_gpu_source"] != "nvml" {
						return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("invalid exporter process identity"))
					}
					allowed := false
					for _, scope := range clusters[cluster] {
						if scope.NodeID == w.Metric["ani_node_id"] && scope.NodeUID == w.Metric["ani_node_uid"] {
							allowed = true
						}
					}
					if !allowed {
						return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("unexpected exporter process node"))
					}
					points, e := gpuPoints(w, q.Latest, start, end, m.Base)
					if e != nil {
						return nil, e
					}
					if q.Budget != nil {
						if e := q.Budget.Count(len(points)); e != nil {
							return nil, e
						}
					}
					key := gpuProcessPath(w.Metric)
					if processStarts[key] == nil {
						processStarts[key] = map[int64]time.Time{}
					}
					for _, pt := range points {
						if math.IsNaN(pt.value) || math.IsInf(pt.value, 0) || pt.value < 0 || pt.value > float64(pt.at.Unix()) {
							return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("invalid exporter process start"))
						}
						sec, frac := math.Modf(pt.value)
						processStarts[key][pt.at.Unix()] = time.Unix(int64(sec), int64(frac*1e9)).UTC()
					}
				}
			}
			paired := map[string]map[int64]float64{}
			for _, wire := range stamps {
				identity, _, err := gpuIdentity(wire.Metric, cluster, m, clusters[cluster])
				if err != nil {
					return nil, err
				}
				path := gpuPath(wire.Metric)
				key := identity + "\x00" + path
				if paired[key] == nil {
					paired[key] = map[int64]float64{}
				}
				points, err := gpuPoints(wire, q.Latest, start, end, m.Base)
				if err != nil {
					return nil, err
				}
				processed += len(points)
				if q.Budget != nil {
					if err := q.Budget.Count(len(points)); err != nil {
						return nil, err
					}
				}
				for _, pt := range points {
					if old, exists := paired[key][pt.at.Unix()]; exists && old != pt.value {
						return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("conflicting timestamp pair"))
					}
					paired[key][pt.at.Unix()] = pt.value
				}
			}
			for _, wire := range values {
				if wire.Metric["__name__"] != m.Record {
					return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("unexpected GPU metric name"))
				}
				identity, scope, err := gpuIdentity(wire.Metric, cluster, m, clusters[cluster])
				if err != nil {
					return nil, err
				}
				unique[identity] = true
				if q.Budget != nil {
					if err := q.Budget.ObserveSource(identity); err != nil {
						return nil, err
					}
				}
				if len(unique) > 128 {
					return nil, gpu.Fail("GPU_BUDGET_EXCEEDED", errors.New("more than 128 returned sources"))
				}
				points, err := gpuPoints(wire, q.Latest, start, end, m.Base)
				if err != nil {
					return nil, err
				}
				processed += len(points)
				if q.Budget != nil {
					if err := q.Budget.Count(len(points)); err != nil {
						return nil, err
					}
				}
				if processed > 12000000 {
					return nil, gpu.Fail("GPU_BUDGET_EXCEEDED", errors.New("sample budget exceeded"))
				}
				source := gpu.Source{Key: identity, Scope: scope, Dimension: wire.Metric["xid"]}
				if m.Event {
					if _, e := strconv.ParseUint(source.Dimension, 10, 32); e != nil {
						return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("invalid Xid event code"))
					}
				}
				pair := paired[identity+"\x00"+gpuPath(wire.Metric)]
				for _, pt := range points {
					stamp, ok := pair[pt.at.Unix()]
					if !ok || math.IsNaN(stamp) || math.IsInf(stamp, 0) {
						return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("missing original sample timestamp"))
					}
					sec, frac := math.Modf(stamp)
					processStart := processStarts[gpuProcessPath(wire.Metric)][pt.at.Unix()]
					if m.Event && processSampleTimes[gpuProcessPath(wire.Metric)][pt.at.Unix()] != stamp {
						processStart = time.Time{}
					}
					source.Samples = append(source.Samples, gpu.Sample{Timestamp: pt.at, SampleTime: time.Unix(int64(sec), int64(frac*1e9)).UTC(), Value: pt.value, ProcessStart: processStart})
				}
				all = append(all, source)
			}
			if q.Latest {
				break
			}
			start = start.Add(6 * time.Hour)
		}
	}
	return all, nil
}
func gpuExpression(scopes []gpu.Scope, m gpu.Metric) string {
	unique := map[string]bool{}
	parts := []string{}
	for _, s := range scopes {
		fields := map[string]string{"ani_cluster_id": s.ClusterID, "ani_node_id": s.NodeID, "ani_node_uid": s.NodeUID, "ani_gpu_source": m.Profile}
		if m.Profile == "nvml" {
			fields["uuid"] = gpu.NormalizeUUID(s.GPUUUID)
		} else {
			fields["podnamespace"] = s.Namespace
			fields["podname"] = s.PodName
			fields["ctrname"] = s.ContainerName
			fields["deviceuuid"] = s.GPUUUID
			if s.VDeviceID != "" {
				fields["vdeviceid"] = s.VDeviceID
			}
		}
		keys := []string{}
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		selectors := []string{}
		for _, k := range keys {
			v := fields[k]
			if k == "deviceuuid" {
				selectors = append(selectors, k+"=~"+strconv.Quote("(?i)(GPU-)?"+regexp.QuoteMeta(gpu.NormalizeUUID(v))))
			} else {
				selectors = append(selectors, k+"="+strconv.Quote(v))
			}
		}
		part := m.Record + "{" + strings.Join(selectors, ",") + "}"
		if m.Profile == "nvml" && !m.Event {
			profile := fmt.Sprintf(`ani_cluster_id=%s,ani_node_id=%s,ani_node_uid=%s,ani_gpu_source="nvml"`, strconv.Quote(s.ClusterID), strconv.Quote(s.NodeID), strconv.Quote(s.NodeUID))
			health := `nvidia_smi_last_collect_success{` + profile + `}`
			last := `nvidia_smi_last_collect_success_timestamp_seconds{` + profile + `}`
			part = `(` + part + ` and on(ani_cluster_id,ani_node_id,ani_node_uid,job,instance) (` + health + ` == 1) and on(ani_cluster_id,ani_node_id,ani_node_uid,job,instance) ((time() - ` + last + ` >= 0) and (time() - ` + last + ` <= 45)))`
		}
		if !unique[part] {
			unique[part] = true
			parts = append(parts, part)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, " or ")
}
func (s *source) gpuRequest(parent context.Context, q gpu.Query, m gpu.Metric, expr string, start, end time.Time) ([]wireSeries, error) {
	select {
	case gpuSlots <- struct{}{}:
	case <-parent.Done():
		return nil, gpu.Fail("GPU_QUERY_TIMEOUT", parent.Err())
	}
	defer func() { <-gpuSlots }()
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	address := *s.base
	endpoint := "/api/v1/query_range"
	form := url.Values{"query": {expr}, "timeout": {"10s"}, "limit": {"129"}}
	if q.Latest {
		endpoint = "/api/v1/query"
		form.Set("time", strconv.FormatInt(q.Start.Unix(), 10))
	} else {
		form.Set("start", strconv.FormatInt(start.Unix(), 10))
		form.Set("end", strconv.FormatInt(end.Unix(), 10))
		form.Set("step", strconv.FormatInt(int64(m.Base/time.Second), 10))
	}
	address.Path = strings.TrimRight(address.Path, "/") + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address.String(), io.NopCloser(strings.NewReader(form.Encode())))
	if err != nil {
		return nil, gpu.Fail("GPU_QUERY_FAILED", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	res, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, gpu.Fail("GPU_QUERY_TIMEOUT", ctx.Err())
		}
		return nil, gpu.Fail("GPU_SOURCE_UNAVAILABLE", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, gpuMaxBody+1))
	if err != nil {
		return nil, gpu.Fail("GPU_SOURCE_UNAVAILABLE", err)
	}
	if len(body) > gpuMaxBody {
		return nil, gpu.Fail("GPU_BUDGET_EXCEEDED", errors.New("upstream response exceeds 16 MiB"))
	}
	var wire envelope
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", err)
	}
	if wire.Status == "error" && wire.ErrorType == "timeout" {
		return nil, gpu.Fail("GPU_QUERY_TIMEOUT", errors.New("upstream timeout"))
	}
	if res.StatusCode >= 500 {
		return nil, gpu.Fail("GPU_SOURCE_UNAVAILABLE", fmt.Errorf("upstream HTTP %d", res.StatusCode))
	}
	if res.StatusCode != 200 || wire.Status != "success" {
		return nil, gpu.Fail("GPU_QUERY_FAILED", errors.New("upstream rejected fixed GPU query"))
	}
	want := "matrix"
	if q.Latest {
		want = "vector"
	}
	if wire.Data.ResultType != want || wire.Data.Result == nil || len(wire.Warnings) > 0 || len(wire.Infos) > 0 {
		return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("incomplete upstream response"))
	}
	if len(wire.Data.Result) >= 129 {
		return nil, gpu.Fail("GPU_BUDGET_EXCEEDED", errors.New("upstream source cap reached"))
	}
	return wire.Data.Result, nil
}
func gpuIdentity(labels map[string]string, cluster string, m gpu.Metric, scopes []gpu.Scope) (string, gpu.Scope, error) {
	fail := func() (string, gpu.Scope, error) {
		return "", gpu.Scope{}, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("untrusted GPU source labels"))
	}
	if labels["ani_cluster_id"] != cluster || labels["ani_gpu_source"] != m.Profile || labels["ani_node_id"] == "" || labels["ani_node_uid"] == "" {
		return fail()
	}
	s := gpu.Scope{ClusterID: cluster, NodeID: labels["ani_node_id"], NodeUID: labels["ani_node_uid"]}
	if m.Profile == "nvml" {
		s.GPUUUID = gpu.NormalizeUUID(labels["uuid"])
	} else {
		s.GPUUUID = gpu.NormalizeUUID(labels["deviceuuid"])
		s.Namespace = labels["podnamespace"]
		s.PodName = labels["podname"]
		s.ContainerName = labels["ctrname"]
		s.VDeviceID = labels["vdeviceid"]
	}
	if s.GPUUUID == "" {
		return fail()
	}
	found := false
	for _, allowed := range scopes {
		if allowed.NodeID != s.NodeID || allowed.NodeUID != s.NodeUID || gpu.NormalizeUUID(allowed.GPUUUID) != s.GPUUUID {
			continue
		}
		if m.Profile == "hami" && (allowed.Namespace != s.Namespace || allowed.PodName != s.PodName || allowed.ContainerName != s.ContainerName || (allowed.VDeviceID != "" && allowed.VDeviceID != s.VDeviceID)) {
			continue
		}
		found = true
		break
	}
	if !found {
		return fail()
	}
	return gpu.SourceKey(s, m.Profile == "nvml"), s, nil
}
func gpuPath(labels map[string]string) string {
	keys := []string{}
	for k := range labels {
		if k != "__name__" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, "\x00")
}
func gpuPoints(w wireSeries, latest bool, start, end time.Time, base time.Duration) ([]gpuWireSample, error) {
	fail := func() ([]gpuWireSample, error) {
		return nil, gpu.Fail("GPU_SOURCE_RESPONSE_INVALID", errors.New("invalid GPU sample tuple or grid"))
	}
	if len(w.Histogram) > 0 || len(w.Histograms) > 0 {
		return fail()
	}
	raw := w.Values
	if latest {
		if len(w.Value) == 0 || len(w.Values) > 0 {
			return fail()
		}
		raw = []json.RawMessage{w.Value}
	} else if len(w.Value) > 0 || w.Values == nil {
		return fail()
	}
	out := []gpuWireSample{}
	seen := map[int64]bool{}
	for _, pt := range raw {
		var tuple []json.RawMessage
		if json.Unmarshal(pt, &tuple) != nil || len(tuple) != 2 {
			return fail()
		}
		var sec float64
		var val string
		if json.Unmarshal(tuple[0], &sec) != nil || json.Unmarshal(tuple[1], &val) != nil || math.IsNaN(sec) || math.IsInf(sec, 0) || math.Trunc(sec) != sec {
			return fail()
		}
		at := time.Unix(int64(sec), 0).UTC()
		if at.Before(start) || at.After(end) || (!latest && at.Unix()%int64(base/time.Second) != 0) || seen[at.Unix()] {
			return fail()
		}
		seen[at.Unix()] = true
		v, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fail()
		}
		out = append(out, gpuWireSample{at, v})
	}
	return out, nil
}

func gpuProcessExpression(scopes []gpu.Scope) string {
	seen := map[string]bool{}
	parts := []string{}
	for _, s := range scopes {
		selector := fmt.Sprintf(`process_start_time_seconds{ani_cluster_id=%s,ani_node_id=%s,ani_node_uid=%s,ani_gpu_source="nvml"}`, strconv.Quote(s.ClusterID), strconv.Quote(s.NodeID), strconv.Quote(s.NodeUID))
		if !seen[selector] {
			seen[selector] = true
			parts = append(parts, selector)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, " or ")
}
func gpuProcessPath(labels map[string]string) string {
	out := map[string]string{}
	for k, v := range labels {
		if k != "uuid" && k != "xid" {
			out[k] = v
		}
	}
	return gpuPath(out)
}
