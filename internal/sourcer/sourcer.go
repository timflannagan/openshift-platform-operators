package sourcer

import (
	"context"
	"fmt"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
)

type Bundle struct {
	Version  string
	Image    string
	Replaces string
	Skips    []string
}

func (b Bundle) String() string {
	return fmt.Sprintf("Version: %s; Image: %s; Replaces %s", b.Version, b.Image, b.Replaces)
}

type Sourcer interface {
	Source(context.Context, *platformv1alpha1.PlatformOperator) (*Bundle, error)
}

func NewNoopSourcer() *noopSourcer {
	return &noopSourcer{}
}

type noopSourcer struct {
	b *Bundle
}

func (no *noopSourcer) SetBundle(b *Bundle) {
	no.b = b
}

func (no *noopSourcer) Source(context.Context, *platformv1alpha1.PlatformOperator) (*Bundle, error) {
	return nil, nil
}
