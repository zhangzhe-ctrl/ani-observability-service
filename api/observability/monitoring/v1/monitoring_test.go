package monitoringv1

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestMetricPointPreservesZeroAndAbsence(t *testing.T) {
	zero := float64(0)
	for _, point := range []*MetricPoint{{Value: &zero}, {GapReason: GapReason_GAP_REASON_NO_DATA}} {
		data, err := proto.Marshal(point)
		if err != nil {
			t.Fatal(err)
		}
		var decoded MetricPoint
		if err := proto.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if (point.Value == nil) != (decoded.Value == nil) {
			t.Fatal("value presence changed in protobuf round trip")
		}
		if decoded.Value != nil && *decoded.Value != 0 {
			t.Fatal("measured zero changed")
		}
	}
}
