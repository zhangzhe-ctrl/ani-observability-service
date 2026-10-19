# Gov 公共 DTO 增量

下列消息追加到既有 `api/protos/instance/service/v1/instance.proto`，沿用其 package、go_package 和生成链。新增消息的字段编号固定如下；不要重建现有文件或改既有字段。

```proto
message QueryInstanceMetricsRequest {
  string instance_id = 1 [json_name = "instance_id"];
  QueryInstanceMetricsData data = 2;
}

message QueryInstanceMetricsData {
  repeated string metric_codes = 1 [json_name = "metric_codes"];
  google.protobuf.Timestamp start = 2;
  google.protobuf.Timestamp end = 3;
  optional uint32 step_seconds = 4 [json_name = "step_seconds"];
}

message QueryInstanceMetricsResponse {
  InstanceMetricsRangeData data = 1;
}

message InstanceMetricsRangeData {
  string instance_id = 1 [json_name = "instance_id"];
  google.protobuf.Timestamp start = 2;
  google.protobuf.Timestamp end = 3;
  uint32 step_seconds = 4 [json_name = "step_seconds"];
  repeated InstanceMetricSeries series = 5;
}

message GetInstanceLatestMetricsResponse {
  InstanceMetricsLatestData data = 1;
}

message InstanceMetricsLatestData {
  string instance_id = 1 [json_name = "instance_id"];
  google.protobuf.Timestamp evaluated_at = 2 [json_name = "evaluated_at"];
  repeated InstanceMetricSeries series = 3;
}

message InstanceMetricSeries {
  string metric_code = 1 [json_name = "metric_code"];
  string unit = 2;
  string status = 3;
  repeated InstanceMetricPoint points = 4;
}

message InstanceMetricPoint {
  google.protobuf.Timestamp timestamp = 1;
  google.protobuf.DoubleValue value = 2;
  optional string gap_reason = 3 [json_name = "gap_reason"];
}
```

复用/补齐 Timestamp 与 wrappers.proto import。latest 请求复用既有 GetInstanceRequest。

在 `api/protos/admin/service/v1/i_instance.proto` 的既有公共 service 增加 QueryInstanceMetrics、GetInstanceLatestMetrics，使用所在文件既有 import 和类型限定方式。HTTP annotation 为：

```proto
// QueryInstanceMetrics
option (google.api.http) = {
  post: "/api/v1/instances/{instance_id}/metrics/query"
  body: "*"
};

// GetInstanceLatestMetrics
option (google.api.http) = {
  get: "/api/v1/instances/{instance_id}/metrics/latest"
};
```

annotation 生成文档；运行时薄 handler 保证HTTP200、数据包装、严格JSON输入、Operation和Middleware。若生成注册器已注册相同路径，需采用现有模式使每条路由只注册一次，不改变其他实例接口。

虽然请求 proto 有路径字段 instance_id，公共body中该字段必须拒绝，由路径绑定；不能仅凭proto结构接受它。normal point的gap_reason缺省，value为包装的0时输出0、nil时输出null；OpenAPI生成结果须标记nullable并由真实codec验证。

这些字段映射为本包实施定义，示例见 `../examples/`，限制/错误见 `../CONTRACT.md`。
