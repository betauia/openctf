// Package instances spawns and manages per-challenge containers behind
// /api/challenges/{cid}/instance and /api/admin/instances.
package instances

import (
	"context"
	"errors"
	"fmt"
	"uuid"
)

// Errors returned by a Provider.
var (
	ErrNotFound       = errors.New("instance not found")
	ErrNotImplemented = errors.New("not implemented for this provider")
)

// Instance is a running challenge container.
type Instance struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Image       string     `json:"image"`
	Status      string     `json:"status"`
	CreatedAt   string     `json:"created_at"`
	HostIP      string     `json:"host_ip,omitzero"`
	HostPort    string     `json:"host_port,omitzero"`
	ChallengeID *uuid.UUID `json:"challenge_id,omitzero"`
	Provider    string     `json:"provider"`
}

// Provider runs challenge containers. Get and Delete return ErrNotFound for
// unknown ids and for containers that aren't challenge instances.
type Provider interface {
	Create(ctx context.Context, challengeID uuid.UUID, image string, port int) (Instance, error)
	List(ctx context.Context) ([]Instance, error)
	Get(ctx context.Context, id string) (Instance, error)
	Delete(ctx context.Context, id string) error
}

// NewProvider returns the provider named by INSTANCE_PROVIDER.
func NewProvider(name, hostDomain string) (Provider, error) {
	switch name {
	case "docker":
		return NewDocker(hostDomain)
	case "kubernetes":
		return Kubernetes{}, nil
	default:
		return nil, fmt.Errorf("unknown INSTANCE_PROVIDER %q", name)
	}
}
