package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	monitoring "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/monitoring"
	usage "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/usage"
)

// usagePodView retains tenant identity across the three existing owner-label
// conventions. Conflicting assignments are excluded rather than counted twice.
func usageRawPodView(ids []string) string {
	ids = append([]string{}, ids...)
	sort.Strings(ids)
	matcher := strconv.Quote(strings.Join(ids, "|"))
	view := `max by (namespace,pod,usage_tenant) (` +
		`label_replace((kube_pod_labels{label_ani_io_tenant_id=~` + matcher + `} and (time()-timestamp(kube_pod_labels)<=45)),"usage_tenant","$1","label_ani_io_tenant_id","(.*)")` +
		` or label_replace((kube_pod_labels{label_ani_kubercloud_com_tenant_id=~` + matcher + `} and (time()-timestamp(kube_pod_labels)<=45)),"usage_tenant","$1","label_ani_kubercloud_com_tenant_id","(.*)")` +
		` or ((kube_pod_labels and (time()-timestamp(kube_pod_labels)<=45)) * on(namespace) group_left(usage_tenant) max by(namespace,usage_tenant) (` +
		`label_replace((kube_namespace_labels{label_ani_io_resource_tenant_id=~` + matcher + `} and (time()-timestamp(kube_namespace_labels)<=45)),"usage_tenant","$1","label_ani_io_resource_tenant_id","(.*)"))))`
	return view
}
func usagePodView(ids []string) string {
	view := usageRawPodView(ids)
	return `(` + view + ` and on(namespace,pod) (count by(namespace,pod) (` + view + `)==1))`
}
func usageExpression(q usage.PodQuery) string {
	view := usagePodView(q.Tenants)
	join := ` * on(namespace,pod) group_left(usage_tenant) ` + view
	parts := []string{}
	add := func(stat, expr string) {
		parts = append(parts, `label_replace((`+expr+`),"usage_metric","`+stat+`","__name__",".*")`)
	}
	running := `(kube_pod_container_status_running==1) and (time()-timestamp(kube_pod_container_status_running)<=45)`
	seeds := []string{}
	for _, id := range q.Tenants {
		seeds = append(seeds, `label_replace(vector(0),"usage_tenant",`+strconv.Quote(id)+`,"__name__",".*")`)
	}
	// A live unrelated Pod is insufficient zero evidence: the owner-label and
	// running-container inventory metric families must both be present and fresh.
	inventory := `((count(kube_pod_info and (time()-timestamp(kube_pod_info)<=45))>0)` +
		` and (count(kube_pod_labels and (time()-timestamp(kube_pod_labels)<=45))>0)` +
		` and (count(kube_pod_container_status_running and (time()-timestamp(kube_pod_container_status_running)<=45))>0))`
	zeros := `((` + strings.Join(seeds, " or ") + `) and on() ` + inventory + `)`
	expected := `(count by(usage_tenant) ((max by(namespace,pod,container) (` + running + `))` + join + `) or on(usage_tenant) ` + zeros + `)`
	add("expected", expected)
	add("inventory", `(`+zeros+`+1)`)
	raw := usageRawPodView(q.Tenants)
	add("ownership", `count by(usage_tenant) (`+raw+`)`)
	conflicts := `count by(usage_tenant) (` + raw + ` and on(namespace,pod) (count by(namespace,pod) (` + raw + `)>1))`
	add("conflicts", `(`+conflicts+` or on(usage_tenant) `+zeros+`)`)
	for _, metric := range q.Metrics {
		record := `container_memory_working_set_bytes{container!="",container!="POD",namespace!="",pod!=""}`
		expr := record
		if metric == "cpu" {
			record = `container_cpu_usage_seconds_total{container!="",container!="POD",namespace!="",pod!=""}`
			expr = `rate(` + record + `[5m])`
		}
		// Match actual observations to the expected running container identity set
		// before counting. Extra containers cannot compensate for missing ones.
		values := `((max by(namespace,pod,container) (` + expr + ` and (time()-timestamp(` + record + `)<=45))) and on(namespace,pod,container) (max by(namespace,pod,container) (` + running + `)))` + join
		add(metric, `sum by(usage_tenant) (`+values+`)`)
		add("observed_"+metric, `count by(usage_tenant) (`+values+`)`)
	}
	return strings.Join(parts, " or ")
}

// QueryUsagePods issues one explicit-cluster grouped query, bounded by the
// trusted tenant set. Counts are evidence about labelled containers only; this
// does not assert complete business ownership when admission labels are absent.
func (p *Prometheus) QueryUsagePods(ctx context.Context, q usage.PodQuery) (usage.PodResult, error) {
	s := p.sources[q.Cluster]
	if s == nil {
		return nil, usage.Fail("USAGE_SOURCE_UNAVAILABLE", "configured cluster datasource required")
	}
	// KSM/cAdvisor owner expressions intentionally use their native labels, not
	// an invented cluster label. A shared endpoint cannot prove cluster scope;
	// querying it once for every configured cluster would duplicate the totals.
	for clusterID, other := range p.sources {
		if clusterID != q.Cluster && strings.TrimRight(other.base.String(), "/") == strings.TrimRight(s.base.String(), "/") {
			return nil, usage.Fail("USAGE_SOURCE_UNAVAILABLE", "shared datasource cannot prove cluster ownership")
		}
	}
	address := *s.base
	address.Path = strings.TrimRight(address.Path, "/") + "/api/v1/query_range"
	form := url.Values{"query": {usageExpression(q)}, "start": {strconv.FormatInt(q.Start.Unix(), 10)}, "end": {strconv.FormatInt(q.End.Unix(), 10)}, "step": {strconv.FormatInt(int64(q.Step.Seconds()), 10)}, "timeout": {"14s"}, "limit": {"1025"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address.String(), io.NopCloser(strings.NewReader(form.Encode())))
	if err != nil {
		return nil, usage.Fail("USAGE_SOURCE_UNAVAILABLE", "construct query")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if s.token != "" {
		request.Header.Set("Authorization", "Bearer "+s.token)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, usage.Fail("USAGE_SOURCE_UNAVAILABLE", "cluster query unavailable")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return nil, usage.Fail("USAGE_SOURCE_UNAVAILABLE", "read cluster query")
	}
	if len(body) > maxBody {
		return nil, usage.Fail("USAGE_BUDGET_EXCEEDED", "cluster response exceeds 8 MiB")
	}
	var wire envelope
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, usage.Fail("USAGE_SOURCE_RESPONSE_INVALID", "invalid response envelope")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || wire.Status == "error" {
		return nil, usage.Fail("USAGE_SOURCE_UNAVAILABLE", "cluster query failed")
	}
	if wire.Status != "success" || wire.Data.ResultType != "matrix" || wire.Data.Result == nil || len(wire.Warnings) > 0 || len(wire.Infos) > 0 {
		return nil, usage.Fail("USAGE_SOURCE_RESPONSE_INVALID", "incomplete response envelope")
	}
	if len(wire.Data.Result) >= 1025 {
		return nil, usage.Fail("USAGE_BUDGET_EXCEEDED", "tenant series budget exceeded")
	}
	validIDs := map[string]bool{}
	out := usage.PodResult{}
	n := int((q.End.Sub(q.Start) + q.Step - 1) / q.Step)
	for _, id := range q.Tenants {
		validIDs[id] = true
		out[id] = map[string][]usage.PodPoint{}
		for _, metric := range q.Metrics {
			rows := make([]usage.PodPoint, n)
			for i := range rows {
				rows[i].Timestamp = q.Start.Add(q.Step * time.Duration(i))
			}
			out[id][metric] = rows
		}
	}
	seen := map[string]bool{}
	allowed := map[string]bool{"expected": true, "inventory": true, "conflicts": true, "ownership": true}
	for _, m := range q.Metrics {
		allowed[m] = true
		allowed["observed_"+m] = true
	}
	for _, series := range wire.Data.Result {
		id, stat := series.Metric["usage_tenant"], series.Metric["usage_metric"]
		key := id + "/" + stat
		if len(series.Metric) != 2 || !validIDs[id] || !allowed[stat] || seen[key] || len(series.Histograms) > 0 || len(series.Histogram) > 0 || len(series.Value) > 0 || series.Values == nil {
			return nil, usage.Fail("USAGE_SOURCE_RESPONSE_INVALID", "unexpected tenant series identity")
		}
		seen[key] = true
		seenTimes := map[int64]bool{}
		for _, raw := range series.Values {
			sample, e := parsePoint(raw, monitoring.Query{Start: q.Start, End: q.End, Step: q.Step})
			if e != nil || math.IsNaN(sample.Value) || math.IsInf(sample.Value, 0) || sample.Value < 0 {
				return nil, usage.Fail("USAGE_SOURCE_RESPONSE_INVALID", fmt.Sprint("invalid statistic sample: ", e))
			}
			if seenTimes[sample.Timestamp.Unix()] {
				return nil, usage.Fail("USAGE_SOURCE_RESPONSE_INVALID", "duplicate statistic timestamp")
			}
			seenTimes[sample.Timestamp.Unix()] = true
			if !sample.Timestamp.Before(q.End) {
				continue
			}
			i := int(sample.Timestamp.Sub(q.Start) / q.Step)
			v := sample.Value
			for _, metric := range q.Metrics {
				rows := out[id][metric]
				switch stat {
				case metric:
					rows[i].Value = &v
				case "expected":
					rows[i].Expected = &v
				case "inventory":
					rows[i].Inventory = &v
				case "conflicts":
					rows[i].Conflicts = &v
				case "ownership":
					rows[i].Ownership = &v
				case "observed_" + metric:
					rows[i].Observed = &v
				}
				out[id][metric] = rows
			}
		}
	}
	for id, metrics := range out {
		for metric, rows := range metrics {
			for i := range rows {
				point := &rows[i]
				if point.Expected != nil && *point.Expected == 0 && point.Inventory != nil && *point.Inventory > 0 && point.Ownership != nil && *point.Ownership > 0 && point.Conflicts != nil && *point.Conflicts == 0 && point.Value == nil {
					zero := float64(0)
					point.Value = &zero
					point.Observed = &zero
				}
			}
			out[id][metric] = rows
		}
	}
	return out, nil
}
