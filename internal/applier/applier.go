package applier

import (
	"context"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Applier interface {
	Apply(context.Context, *platformv1alpha1.PlatformOperator) (client.Object, error)
}
