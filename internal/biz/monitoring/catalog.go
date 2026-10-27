package monitoring

import "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"

type Metric struct{ Code, Record, Unit string }

const CPU = "cpu.usage_cores"
const Memory = "memory.working_set_bytes"
const NetworkReceive = "network.receive_bytes_per_second"
const NetworkTransmit = "network.transmit_bytes_per_second"

// Lookup 按 target kind 返回已登记的指标定义。训练 kind 只登记 CPU 与内存，
// 网络码在训练 kind 下返回未登记，由调用方转为 InvalidQuery。
func Lookup(code string, kind int32) (Metric, bool) {
	switch kind {
	case target.ContainerInstance:
		return lookupInstance(code)
	case target.TrainingExecution:
		return lookupTraining(code)
	default:
		return Metric{}, false
	}
}

func lookupInstance(code string) (Metric, bool) {
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

func lookupTraining(code string) (Metric, bool) {
	switch code {
	case CPU:
		return Metric{CPU, "ani_training_cpu_usage_cores", "cores"}, true
	case Memory:
		return Metric{Memory, "ani_training_memory_working_set_bytes", "bytes"}, true
	default:
		return Metric{}, false
	}
}
