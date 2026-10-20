package monitoring

type Metric struct{ Code, Record, Unit string }

const CPU = "cpu.usage_cores"
const Memory = "memory.working_set_bytes"
const NetworkReceive = "network.receive_bytes_per_second"
const NetworkTransmit = "network.transmit_bytes_per_second"

func Lookup(code string) (Metric, bool) {
	switch code {
	case CPU:
		return Metric{CPU, "ani_instance_cpu_usage_cores", "cores"}, true
	case Memory:
		return Metric{Memory, "ani_instance_memory_working_set_bytes", "bytes"}, true
	case NetworkReceive:
		return Metric{NetworkReceive, "ani_instance_network_receive_bytes_per_second", "bytes_per_second"}, true
	case NetworkTransmit:
		return Metric{NetworkTransmit, "ani_instance_network_transmit_bytes_per_second", "bytes_per_second"}, true
	default:
		return Metric{}, false
	}
}
