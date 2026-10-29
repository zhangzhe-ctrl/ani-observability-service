package target

import "testing"

func validScope() Scope {
	return Scope{Kind: ContainerInstance, TenantID: "11111111-1111-4111-8111-111111111111",
		InstanceID: "22222222-2222-4222-8222-222222222222", ClusterID: "33333333-3333-4333-8333-333333333333",
		Namespace: "workspace-a", ContainerName: "workspace"}
}

func trainingScope() Scope {
	return Scope{Kind: TrainingExecution, TenantID: "11111111-1111-4111-8111-111111111111",
		InstanceID: "22222222-2222-4222-8222-222222222222", ClusterID: "33333333-3333-4333-8333-333333333333",
		Namespace: "ani-modeldev-tenant-a-v21", ContainerName: "node",
		PodUID: "44444444-4444-4444-8444-444444444444"}
}

func vmScope() Scope {
	return Scope{Kind: VM, TenantID: "11111111-1111-4111-8111-111111111111",
		InstanceID: "22222222-2222-4222-8222-222222222222", ClusterID: "33333333-3333-4333-8333-333333333333",
		Namespace: "tenant-64c5c35a-f554-432c-b4bc-7b4f36f08420", ContainerName: "compute"}
}

func TestValidateAcceptsBothKinds(t *testing.T) {
	if err := validScope().Validate(); err != nil {
		t.Fatalf("instance scope rejected: %v", err)
	}
	if err := trainingScope().Validate(); err != nil {
		t.Fatalf("training scope rejected: %v", err)
	}
	step := trainingScope()
	step.ContainerName = "main"
	if err := step.Validate(); err != nil {
		t.Fatalf("training step container rejected: %v", err)
	}
	if err := vmScope().Validate(); err != nil {
		t.Fatalf("vm scope rejected: %v", err)
	}
}

func TestValidateVMScope(t *testing.T) {
	rejections := []struct {
		name   string
		mutate func(*Scope)
	}{
		{"workspace container", func(s *Scope) { s.ContainerName = "workspace" }},
		{"pod uid", func(s *Scope) { s.PodUID = "44444444-4444-4444-8444-444444444444" }},
		{"bad instance", func(s *Scope) { s.InstanceID = "not-a-uuid" }},
		{"bad cluster", func(s *Scope) { s.ClusterID = "not-a-uuid" }},
		{"bad namespace", func(s *Scope) { s.Namespace = "Tenant_Upper" }},
	}
	for _, tc := range rejections {
		t.Run(tc.name, func(t *testing.T) {
			s := vmScope()
			tc.mutate(&s)
			if err := s.Validate(); err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
		})
	}
}

func TestValidateRejectsKindMismatch(t *testing.T) {
	cases := []struct {
		name   string
		kind   int32
		mutate func(*Scope)
	}{
		{"instance pod uid", ContainerInstance, func(s *Scope) { s.PodUID = "44444444-4444-4444-8444-444444444444" }},
		{"instance container", ContainerInstance, func(s *Scope) { s.ContainerName = "main" }},
		{"training no pod", TrainingExecution, func(s *Scope) { s.PodUID = "" }},
		{"training bad pod", TrainingExecution, func(s *Scope) { s.PodUID = "not-a-uuid" }},
		{"training upper", TrainingExecution, func(s *Scope) { s.ContainerName = "Main" }},
		{"training namespace", TrainingExecution, func(s *Scope) { s.Namespace = "ani_modeldev" }},
		{"unknown kind", 99, func(*Scope) {}},
		{"zero kind", 0, func(*Scope) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validScope()
			if tc.kind == TrainingExecution {
				s = trainingScope()
			}
			s.Kind = tc.kind
			tc.mutate(&s)
			if err := s.Validate(); err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
		})
	}
}

func tenantScope() Scope {
	return Scope{Kind: TENANT, TenantID: "11111111-1111-4111-8111-111111111111",
		ClusterID: "33333333-3333-4333-8333-333333333333"}
}

func TestValidateTenantScope(t *testing.T) {
	if err := tenantScope().Validate(); err != nil {
		t.Fatalf("tenant scope with cluster rejected: %v", err)
	}
	withoutCluster := tenantScope()
	withoutCluster.ClusterID = ""
	if err := withoutCluster.Validate(); err != nil {
		t.Fatalf("tenant scope without cluster rejected: %v", err)
	}
	rejections := []struct {
		name   string
		mutate func(*Scope)
	}{
		{"instance id", func(s *Scope) { s.InstanceID = "22222222-2222-4222-8222-222222222222" }},
		{"namespace", func(s *Scope) { s.Namespace = "tenant-workload" }},
		{"container", func(s *Scope) { s.ContainerName = "workspace" }},
		{"pod uid", func(s *Scope) { s.PodUID = "44444444-4444-4444-8444-444444444444" }},
		{"bad tenant", func(s *Scope) { s.TenantID = "00000000-0000-0000-0000-000000000000" }},
		{"bad cluster", func(s *Scope) { s.ClusterID = "not-a-uuid" }},
	}
	for _, tc := range rejections {
		t.Run(tc.name, func(t *testing.T) {
			s := tenantScope()
			tc.mutate(&s)
			if err := s.Validate(); err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
		})
	}
}
