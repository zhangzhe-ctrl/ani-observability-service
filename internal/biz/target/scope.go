// Package target defines the trusted resource-owner scope.
package target

import (
	"fmt"
	"regexp"

	"github.com/google/uuid"
)

const (
	ContainerInstance = 1
	TrainingExecution = 2
)

var namespacePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

var containerPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

type Scope struct {
	Kind          int32
	TenantID      string
	InstanceID    string
	ClusterID     string
	Namespace     string
	ContainerName string
	// PodUID 仅 TRAINING_EXECUTION 使用；CONTAINER_INSTANCE 必须留空。
	PodUID string
}

func ValidUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func (s Scope) Validate() error {
	if s.Kind != ContainerInstance && s.Kind != TrainingExecution {
		return fmt.Errorf("invalid target kind")
	}
	if !ValidUUID(s.TenantID) || !ValidUUID(s.InstanceID) || !ValidUUID(s.ClusterID) {
		return fmt.Errorf("invalid canonical UUID")
	}
	if len(s.Namespace) > 63 || !namespacePattern.MatchString(s.Namespace) ||
		len(s.ContainerName) > 63 || !containerPattern.MatchString(s.ContainerName) {
		return fmt.Errorf("invalid namespace or container")
	}
	switch s.Kind {
	case ContainerInstance:
		if s.ContainerName != "workspace" || s.PodUID != "" {
			return fmt.Errorf("invalid container or pod for the instance target")
		}
	case TrainingExecution:
		if !ValidUUID(s.PodUID) {
			return fmt.Errorf("pod uid is required for the training target")
		}
	}
	return nil
}
