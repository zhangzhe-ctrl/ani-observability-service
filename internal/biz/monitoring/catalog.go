package monitoring

type Metric struct{ Code, Record, Unit string }

const CPU = "cpu.usage_cores"
const Memory = "memory.working_set_bytes"

func Lookup(code string) (Metric, bool) {
	switch code {
	case CPU:
		return Metric{CPU, "ani_instance_cpu_usage_cores", "cores"}, true
	case Memory:
		return Metric{Memory, "ani_instance_memory_working_set_bytes", "bytes"}, true
	default:
		return Metric{}, false
	}
}
