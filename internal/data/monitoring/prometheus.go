package monitoring

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/monitoring"
	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"
)

const maxBody = 8 * 1024 * 1024

var cidPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Datasource struct{ ClusterID, Address, BearerTokenFile, TLSCAFile, TLSServerName string }
type source struct {
	base      *url.URL
	client    *http.Client
	transport *http.Transport
	token     string
}
type Prometheus struct{ sources map[string]*source }

func New(config []Datasource) (*Prometheus, error) {
	p := &Prometheus{sources: make(map[string]*source)}
	for _, c := range config {
		if !target.ValidUUID(c.ClusterID) || p.sources[c.ClusterID] != nil {
			p.Close()
			return nil, fmt.Errorf("invalid or duplicate datasource cluster ID")
		}
		base, err := url.Parse(c.Address)
		if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
			p.Close()
			return nil, fmt.Errorf("invalid datasource address")
		}
		cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: c.TLSServerName}
		if c.TLSCAFile != "" {
			pem, err := os.ReadFile(c.TLSCAFile)
			if err != nil {
				p.Close()
				return nil, fmt.Errorf("read datasource CA: %w", err)
			}
			roots := x509.NewCertPool()
			if !roots.AppendCertsFromPEM(pem) {
				p.Close()
				return nil, fmt.Errorf("invalid datasource CA")
			}
			cfg.RootCAs = roots
		}
		if base.Scheme == "http" && (c.TLSCAFile != "" || c.TLSServerName != "") {
			p.Close()
			return nil, fmt.Errorf("TLS configured for HTTP datasource")
		}
		token := ""
		if c.BearerTokenFile != "" {
			data, err := os.ReadFile(c.BearerTokenFile)
			if err != nil {
				p.Close()
				return nil, fmt.Errorf("read datasource credential: %w", err)
			}
			token = strings.TrimSpace(string(data))
			if token == "" || strings.ContainsAny(token, "\r\n") {
				p.Close()
				return nil, fmt.Errorf("invalid datasource credential")
			}
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = cfg
		transport.MaxConnsPerHost = 32
		transport.MaxIdleConnsPerHost = 16
		transport.IdleConnTimeout = 30 * time.Second
		client := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		p.sources[c.ClusterID] = &source{base: base, client: client, transport: transport, token: token}
	}
	return p, nil
}

func (p *Prometheus) Close() {
	for _, s := range p.sources {
		s.transport.CloseIdleConnections()
	}
}

type wireSeries struct {
	Metric     map[string]string `json:"metric"`
	Values     []json.RawMessage `json:"values"`
	Value      json.RawMessage   `json:"value"`
	Histograms json.RawMessage   `json:"histograms"`
	Histogram  json.RawMessage   `json:"histogram"`
}
type envelope struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string       `json:"resultType"`
		Result     []wireSeries `json:"result"`
	} `json:"data"`
	ErrorType string   `json:"errorType"`
	Error     string   `json:"error"`
	Warnings  []string `json:"warnings"`
	Infos     []string `json:"infos"`
}

func (p *Prometheus) Query(ctx context.Context, q biz.Query, m biz.Metric) ([]biz.Source, error) {
	expr, err := Expression(q.Target, m)
	if err != nil {
		return nil, err
	}
	s := p.sources[q.Target.ClusterID]
	if s == nil && q.Target.ClusterID == "" && q.Target.Kind == target.TENANT && len(p.sources) == 1 {
		// 租户级 target 的 ClusterID 可选：单数据源部署直接解析，多数据源必须显式指定。
		for _, only := range p.sources {
			s = only
		}
	}
	if s == nil {
		return nil, biz.Fail(biz.SourceNotConfigured, fmt.Errorf("no configured datasource"))
	}
	address := *s.base
	endpoint := "/api/v1/query_range"
	// 实例/训练沿用 4s 求值超时；租户级长窗口放宽到 14s（批次预算 15s 内留余量）。
	prometheusTimeout := "4s"
	if q.Target.Kind == target.TENANT {
		prometheusTimeout = "14s"
	}
	form := url.Values{"query": {expr}, "timeout": {prometheusTimeout}, "limit": {"129"}}
	if q.Latest {
		endpoint = "/api/v1/query"
		form.Set("time", strconv.FormatInt(q.Start.Unix(), 10))
	} else {
		form.Set("start", strconv.FormatInt(q.Start.Unix(), 10))
		form.Set("end", strconv.FormatInt(q.End.Unix(), 10))
		form.Set("step", strconv.FormatInt(int64(q.Step/time.Second), 10))
	}
	address.Path = strings.TrimRight(address.Path, "/") + endpoint
	// An unreplayable POST body prevents transport retries after a connection failure.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address.String(), io.NopCloser(strings.NewReader(form.Encode())))
	if err != nil {
		return nil, biz.Fail(biz.QueryFailed, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	response, err := s.client.Do(req)
	if err != nil {
		return nil, requestFailure(ctx, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return nil, requestFailure(ctx, err)
	}
	if len(body) > maxBody {
		return nil, biz.Fail(biz.ResultTooLarge, fmt.Errorf("datasource response exceeds byte limit"))
	}
	var wire envelope
	decodeErr := json.Unmarshal(body, &wire)
	if len(wire.Data.Result) >= 129 {
		return nil, biz.Fail(biz.ResultTooLarge, fmt.Errorf("datasource response exceeds series limit"))
	}
	if decodeErr == nil && wire.Status == "error" && wire.ErrorType == "timeout" {
		return nil, biz.Fail(biz.QueryTimeout, fmt.Errorf("datasource query timeout"))
	}
	if response.StatusCode >= 500 {
		return nil, biz.Fail(biz.SourceUnavailable, fmt.Errorf("datasource HTTP %d", response.StatusCode))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, biz.Fail(biz.QueryFailed, fmt.Errorf("datasource rejected fixed query: HTTP %d", response.StatusCode))
	}
	if decodeErr != nil {
		return nil, biz.Fail(biz.SourceResponseInvalid, fmt.Errorf("decode datasource response: %w", decodeErr))
	}
	if wire.Status == "error" {
		return nil, biz.Fail(biz.QueryFailed, fmt.Errorf("datasource query %s: %s", wire.ErrorType, wire.Error))
	}
	wantType := "matrix"
	if q.Latest {
		wantType = "vector"
	}
	if wire.Status != "success" || wire.Data.ResultType != wantType || wire.Data.Result == nil || len(wire.Warnings) > 0 {
		return nil, biz.Fail(biz.SourceResponseInvalid, fmt.Errorf("invalid or incomplete datasource result envelope"))
	}
	out := make([]biz.Source, 0, len(wire.Data.Result))
	identities := make(map[string]bool)
	for _, item := range wire.Data.Result {
		if !validLabels(item.Metric, q.Target, m) {
			return nil, biz.Fail(biz.SourceResponseInvalid, fmt.Errorf("datasource runtime identity mismatch"))
		}
		identity := item.Metric["pod_uid"] + "/" + item.Metric["runtime_container_id"]
		if identities[identity] {
			return nil, biz.Fail(biz.SourceResponseInvalid, fmt.Errorf("duplicate datasource runtime identity"))
		}
		identities[identity] = true
		if len(item.Histograms) > 0 || len(item.Histogram) > 0 {
			return nil, biz.Fail(biz.SourceResponseInvalid, fmt.Errorf("unexpected histogram"))
		}
		points := item.Values
		if q.Latest {
			if len(item.Values) > 0 || len(item.Value) == 0 {
				return nil, biz.Fail(biz.SourceResponseInvalid, fmt.Errorf("invalid vector sample structure"))
			}
			points = []json.RawMessage{item.Value}
		} else if len(item.Value) > 0 || item.Values == nil {
			return nil, biz.Fail(biz.SourceResponseInvalid, fmt.Errorf("invalid matrix sample structure"))
		}
		source := biz.Source{PodUID: item.Metric["pod_uid"], ContainerID: item.Metric["runtime_container_id"]}
		seen := make(map[int64]bool)
		for _, raw := range points {
			point, err := parsePoint(raw, q)
			if err != nil {
				return nil, biz.Fail(biz.SourceResponseInvalid, err)
			}
			if seen[point.Timestamp.Unix()] {
				return nil, biz.Fail(biz.SourceResponseInvalid, fmt.Errorf("duplicate datasource sample time"))
			}
			seen[point.Timestamp.Unix()] = true
			source.Samples = append(source.Samples, point)
		}
		out = append(out, source)
	}
	return out, nil
}

func requestFailure(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return biz.Fail(biz.QueryTimeout, err)
	}
	return biz.Fail(biz.SourceUnavailable, err)
}

func validLabels(labels map[string]string, s target.Scope, m biz.Metric) bool {
	if s.Kind == target.TENANT {
		// 租户级聚合结果没有运行时身份标签：CPU/内存按 ani_tenant_id 聚合后只剩
		// 该标签，GPU 的 sum() 归一无标签。租户锚定由服务端表达式保证，这里只接受
		// 零标签或恰好一个正确的租户标签。
		if len(labels) == 0 {
			return true
		}
		return len(labels) == 1 && labels["ani_tenant_id"] == s.TenantID
	}
	if len(labels) != 8 || labels["__name__"] != m.Record || labels["ani_cluster_id"] != s.ClusterID || labels["ani_tenant_id"] != s.TenantID || labels["ani_instance_id"] != s.InstanceID || labels["namespace"] != s.Namespace || labels["container"] != s.ContainerName {
		return false
	}
	if s.Kind == target.TrainingExecution && labels["pod_uid"] != s.PodUID {
		return false
	}
	return target.ValidUUID(labels["pod_uid"]) && cidPattern.MatchString(labels["runtime_container_id"])
}

func parsePoint(raw json.RawMessage, q biz.Query) (biz.Sample, error) {
	var fields []json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != 2 {
		return biz.Sample{}, fmt.Errorf("invalid datasource point tuple")
	}
	var number any
	if err := json.Unmarshal(fields[0], &number); err != nil {
		return biz.Sample{}, fmt.Errorf("invalid sample timestamp")
	}
	seconds, ok := number.(float64)
	if !ok || math.IsInf(seconds, 0) || math.IsNaN(seconds) || math.Trunc(seconds) != seconds || seconds < float64(q.Start.Unix()) || seconds > float64(q.End.Unix()) {
		return biz.Sample{}, fmt.Errorf("sample timestamp outside range")
	}
	timestamp := time.Unix(int64(seconds), 0).UTC()
	if timestamp.Sub(q.Start)%q.Step != 0 {
		return biz.Sample{}, fmt.Errorf("sample timestamp outside grid")
	}
	var text string
	if err := json.Unmarshal(fields[1], &text); err != nil || text == "" {
		return biz.Sample{}, fmt.Errorf("invalid sample value encoding")
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return biz.Sample{}, fmt.Errorf("invalid sample value")
	}
	return biz.Sample{Timestamp: timestamp, Value: value}, nil
}
