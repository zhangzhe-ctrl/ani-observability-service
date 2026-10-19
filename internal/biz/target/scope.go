// Package target defines the trusted resource-owner scope.
package target

import (
	"fmt"
	"regexp"

	"github.com/google/uuid"
)

const ContainerInstance = 1

var namespacePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

type Scope struct {
	Kind          int32
	TenantID      string
	InstanceID    string
	ClusterID     string
	Namespace     string
	ContainerName string
}

func ValidUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func (s Scope) Validate() error {
	if s.Kind != ContainerInstance || !ValidUUID(s.TenantID) || !ValidUUID(s.InstanceID) || !ValidUUID(s.ClusterID) {
		return fmt.Errorf("invalid target kind or canonical UUID")
	}
	if len(s.Namespace) > 63 || !namespacePattern.MatchString(s.Namespace) || s.ContainerName != "workspace" {
		return fmt.Errorf("invalid namespace or container")
	}
	return nil
}
