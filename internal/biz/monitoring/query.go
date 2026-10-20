package monitoring

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"
)

type Query struct {
	Target     target.Scope
	Codes      []string
	Start, End time.Time
	Step       time.Duration
	Latest     bool
}
type Sample struct {
	Timestamp time.Time
	Value     float64
}
type Source struct {
	PodUID, ContainerID string
	Samples             []Sample
}
type Point struct {
	Timestamp time.Time
	Value     *float64
	GapReason string
}
type Series struct {
	Code, Unit, Status string
	Points             []Point
}
type Result struct {
	Start, End  time.Time
	Step        time.Duration
	EvaluatedAt time.Time
	Series      []Series
}

type Provider interface {
	Query(context.Context, Query, Metric) ([]Source, error)
}

type UseCase struct {
	provider Provider
	now      func() time.Time
	mu       sync.Mutex
	active   int
	tenants  map[string]int
}

func NewUseCase(provider Provider, now func() time.Time) *UseCase {
	if now == nil {
		now = time.Now
	}
	return &UseCase{provider: provider, now: now, tenants: make(map[string]int)}
}

func (u *UseCase) acquire(tenant string) (func(), error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.active >= 16 || u.tenants[tenant] >= 4 {
		return nil, Fail(ConcurrencyLimit, errors.New("request concurrency exhausted"))
	}
	u.active++
	u.tenants[tenant]++
	return func() {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.active--
		u.tenants[tenant]--
		if u.tenants[tenant] == 0 {
			delete(u.tenants, tenant)
		}
	}, nil
}

func (q Query) Validate(now time.Time) error {
	if err := q.Target.Validate(); err != nil {
		return Fail(InvalidQuery, err)
	}
	if len(q.Codes) < 1 || len(q.Codes) > 4 {
		return Fail(InvalidQuery, errors.New("one to four metrics required"))
	}
	seen := make(map[string]bool)
	for _, code := range q.Codes {
		if _, ok := Lookup(code); !ok || seen[code] {
			return Fail(InvalidQuery, errors.New("unknown or duplicate metric"))
		}
		seen[code] = true
	}
	if q.Start.Nanosecond() != 0 || q.End.Nanosecond() != 0 || q.End.Before(q.Start) || q.End.After(now) || q.End.Sub(q.Start) > 24*time.Hour {
		return Fail(InvalidQuery, errors.New("invalid query time window"))
	}
	if q.Step < 30*time.Second || q.Step > time.Hour || q.Step%(30*time.Second) != 0 || q.End.Sub(q.Start)/q.Step+1 > 3000 {
		return Fail(InvalidQuery, errors.New("invalid step or point count"))
	}
	return nil
}

func (u *UseCase) Execute(ctx context.Context, q Query) (*Result, error) {
	now := u.now().UTC().Truncate(time.Second)
	if q.Latest {
		q.Start, q.End, q.Step = now, now, 30*time.Second
	}
	if err := q.Validate(now); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	release, err := u.acquire(q.Target.TenantID)
	if err != nil {
		return nil, err
	}
	defer release()
	batch, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	type item struct {
		index  int
		series Series
		err    error
	}
	results := make(chan item, len(q.Codes))
	for i, code := range q.Codes {
		metric, _ := Lookup(code)
		go func(index int, metric Metric) {
			sources, err := u.provider.Query(batch, q, metric)
			if err != nil {
				results <- item{index: index, err: err}
				return
			}
			series, err := Normalize(q, metric, sources)
			results <- item{index: index, series: series, err: err}
		}(i, metric)
	}
	out := &Result{Start: q.Start.UTC(), End: q.End.UTC(), Step: q.Step, EvaluatedAt: now, Series: make([]Series, len(q.Codes))}
	var first error
	for range q.Codes {
		r := <-results
		if r.err != nil && first == nil {
			first = r.err
			cancel()
		}
		out.Series[r.index] = r.series
	}
	if first != nil {
		if errors.Is(first, context.DeadlineExceeded) {
			return nil, Fail(QueryTimeout, first)
		}
		return nil, first
	}
	if err := batch.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, Fail(QueryTimeout, err)
		}
		return nil, err
	}
	if len(out.Series) != len(q.Codes) {
		return nil, fmt.Errorf("query batch incomplete")
	}
	return out, nil
}
