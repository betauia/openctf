package instances

import (
	"context"
	"uuid"
)

// Kubernetes is a placeholder, like the Python provider it replaces.
type Kubernetes struct{}

// Create returns ErrNotImplemented.
func (Kubernetes) Create(context.Context, uuid.UUID, string, int) (Instance, error) {
	return Instance{}, ErrNotImplemented
}

// List returns ErrNotImplemented.
func (Kubernetes) List(context.Context) ([]Instance, error) { return nil, ErrNotImplemented }

// Get returns ErrNotImplemented.
func (Kubernetes) Get(context.Context, string) (Instance, error) {
	return Instance{}, ErrNotImplemented
}

// Delete returns ErrNotImplemented.
func (Kubernetes) Delete(context.Context, string) error { return ErrNotImplemented }
