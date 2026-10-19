package monitoring

import (
	"fmt"
	"math"
	"time"
)

func Normalize(q Query, metric Metric, sources []Source) (Series, error) {
	n := int(q.End.Sub(q.Start)/q.Step) + 1
	points := make([]Point, n)
	counts := make([]int, n)
	values := make([]float64, n)
	identities := make(map[string]bool)
	for _, source := range sources {
		identity := source.PodUID + "/" + source.ContainerID
		if identities[identity] {
			return Series{}, Fail(SourceResponseInvalid, fmt.Errorf("duplicate runtime series"))
		}
		identities[identity] = true
		seen := make(map[int]bool)
		for _, sample := range source.Samples {
			delta := sample.Timestamp.Sub(q.Start)
			if sample.Timestamp.Before(q.Start) || sample.Timestamp.After(q.End) || delta%q.Step != 0 {
				return Series{}, Fail(SourceResponseInvalid, fmt.Errorf("sample outside requested grid"))
			}
			i := int(delta / q.Step)
			if seen[i] {
				return Series{}, Fail(SourceResponseInvalid, fmt.Errorf("duplicate sample"))
			}
			seen[i] = true
			counts[i]++
			values[i] = sample.Value
		}
	}
	valid := 0
	for i := range points {
		points[i].Timestamp = q.Start.Add(q.Step * time.Duration(i)).UTC()
		switch {
		case counts[i] == 0:
			points[i].GapReason = "NO_DATA"
		case counts[i] > 1:
			points[i].GapReason = "AMBIGUOUS_SOURCE"
		case math.IsNaN(values[i]) || math.IsInf(values[i], 0) || values[i] < 0:
			points[i].GapReason = "INVALID_VALUE"
		default:
			value := values[i]
			points[i].Value = &value
			valid++
		}
	}
	state := "PARTIAL"
	if valid == 0 {
		state = "NO_DATA"
	} else if valid == n {
		state = "OK"
	}
	return Series{Code: metric.Code, Unit: metric.Unit, Status: state, Points: points}, nil
}
