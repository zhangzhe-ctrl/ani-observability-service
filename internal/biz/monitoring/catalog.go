package monitoring

import "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"

type Metric struct{ Code, Record, Unit string }

const CPU = "cpu.usage_cores"
const Memory = "memory.working_set_bytes"
const NetworkReceive = "network.receive_bytes_per_second"
const NetworkTransmit = "network.transmit_bytes_per_second"

// GPU 编码只在容器实例 kind 下登记；VM kind 登记同名编码但不下发数据，
// 使 latest 对 VM 固定返回 6 series 时 GPU 两项稳定为 NO_DATA。
const GPUUtilization = "gpu.utilization_percent"
const GPUMemoryUsage = "gpu.memory_usage_bytes"

// 租户级聚合编码：首页资源趋势专用，表达式由 query_builder 按编码生成。
// 三条编码口径统一为「租户全部工作负载的实际用量」：GPU 由分配份额改为 HAMi 显存实际用量。
const TenantCPU = "tenant.cpu.usage_cores"
const TenantMemory = "tenant.memory.working_set_bytes"
const TenantGPU = "tenant.gpu.memory_usage_bytes"

// Lookup 按 target kind 返回已登记的指标定义。训练 kind 只登记 CPU 与内存，
// 网络码在训练 kind 下返回未登记，由调用方转为 InvalidQuery。
// 租户 kind 只登记三个租户级聚合编码，实例/训练编码互不可见。
func Lookup(code string, kind int32) (Metric, bool) {
	switch kind {
	case target.ContainerInstance:
		return lookupInstance(code)
	case target.TrainingExecution:
		return lookupTraining(code)
	case target.VM:
		return lookupVM(code)
	case target.TENANT:
		return lookupTenant(code)
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
	case GPUUtilization:
		return Metric{GPUUtilization, "ani_instance_gpu_utilization_percent", "percent"}, true
	case GPUMemoryUsage:
		return Metric{GPUMemoryUsage, "ani_instance_gpu_memory_usage_bytes", "bytes"}, true
	default:
		return Metric{}, false
	}
}

// lookupVM 复用实例的 CPU / 内存 / 网络编码，但读取独立的 ani_vm_* 记录。
// GPU 两项没有 VM 采集链路，record 不存在时查询为空，由 Normalize 归为 NO_DATA。
func lookupVM(code string) (Metric, bool) {
	switch code {
	case CPU:
		return Metric{CPU, "ani_vm_cpu_usage_cores", "cores"}, true
	case Memory:
		return Metric{Memory, "ani_vm_memory_working_set_bytes", "bytes"}, true
	case NetworkReceive:
		return Metric{NetworkReceive, "ani_vm_network_receive_bytes_per_second", "bytes_per_second"}, true
	case NetworkTransmit:
		return Metric{NetworkTransmit, "ani_vm_network_transmit_bytes_per_second", "bytes_per_second"}, true
	case GPUUtilization:
		return Metric{GPUUtilization, "ani_vm_gpu_utilization_percent", "percent"}, true
	case GPUMemoryUsage:
		return Metric{GPUMemoryUsage, "ani_vm_gpu_memory_usage_bytes", "bytes"}, true
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

// lookupTenant：三条租户编码都是完整 join 表达式（租户 pod 视图 + 源指标），
// 无法用 record+selector 拼出，Record 仅作文档用途，表达式由
// query_builder.tenantExpression 固定生成。
func lookupTenant(code string) (Metric, bool) {
	switch code {
	case TenantCPU:
		return Metric{TenantCPU, "container_cpu_usage_seconds_total", "cores"}, true
	case TenantMemory:
		return Metric{TenantMemory, "container_memory_working_set_bytes", "bytes"}, true
	case TenantGPU:
		return Metric{TenantGPU, "vGPU_device_memory_usage_in_bytes", "bytes"}, true
	default:
		return Metric{}, false
	}
}
