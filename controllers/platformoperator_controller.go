/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	platformtypes "github.com/openshift/platform-operators/api/v1alpha1"
	"github.com/openshift/platform-operators/internal/applier"
	"github.com/openshift/platform-operators/internal/sourcer"
	"github.com/openshift/platform-operators/internal/util"
)

// PlatformOperatorReconciler reconciles a PlatformOperator object
type PlatformOperatorReconciler struct {
	client.Client
	Sourcer sourcer.Sourcer
	Applier applier.Applier
}

//+kubebuilder:rbac:groups=platform.openshift.io,resources=platformoperators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=platform.openshift.io,resources=platformoperators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=platform.openshift.io,resources=platformoperators/finalizers,verbs=update
//+kubebuilder:rbac:groups=operators.coreos.com,resources=catalogsources,verbs=get;list;watch
//+kubebuilder:rbac:groups=core.rukpak.io,resources=bundledeployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.rukpak.io,resources=bundles,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *PlatformOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContext(ctx)
	log.Info("reconciling request", "req", req.NamespacedName)
	defer log.Info("finished reconciling request", "req", req.NamespacedName)

	po := &platformv1alpha1.PlatformOperator{}
	if err := r.Get(ctx, req.NamespacedName, po); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	defer func() {
		po := po.DeepCopy()
		po.ObjectMeta.ManagedFields = nil

		if err := r.Patch(ctx, po, client.Apply, client.FieldOwner("platformoperator")); err != nil {
			log.Error(err, "failed to patch status")
		}
		if err := r.Status().Patch(ctx, po, client.Apply, client.FieldOwner("platformoperator")); err != nil {
			log.Error(err, "failed to patch status")
		}
	}()

	return r.reconcile(ctx, po)
}

func (r *PlatformOperatorReconciler) reconcile(ctx context.Context, po *platformv1alpha1.PlatformOperator) (ctrl.Result, error) {
	desiredBundle, err := r.ensureDesiredBundle(ctx, po)
	if err != nil {
		meta.SetStatusCondition(&po.Status.Conditions, metav1.Condition{
			Type:    platformtypes.TypeApplied,
			Status:  metav1.ConditionUnknown,
			Reason:  platformtypes.ReasonSourceFailed,
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}
	// TODO: we're never getting here...
	platformtypes.SetDesiredBundle(po, desiredBundle)

	bd, err := r.ensureDesiredBundleDeployment(ctx, po)
	if err != nil {
		meta.SetStatusCondition(&po.Status.Conditions, metav1.Condition{
			Type:    platformtypes.TypeApplied,
			Status:  metav1.ConditionUnknown,
			Reason:  platformtypes.ReasonApplyFailed,
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	// TODO: abstract into a health checker type?
	if failureCond := r.inspectBundleDeploymentStatus(ctx, bd.Status.Conditions); failureCond != nil {
		meta.SetStatusCondition(&po.Status.Conditions, *failureCond)
		return ctrl.Result{}, nil
	}
	meta.SetStatusCondition(&po.Status.Conditions, metav1.Condition{
		Type:    platformtypes.TypeApplied,
		Status:  metav1.ConditionTrue,
		Reason:  platformtypes.ReasonApplySuccessful,
		Message: "Successfully applied the desired olm.bundle content",
	})
	platformtypes.SetActiveBundleDeployment(po, bd.GetName())

	return ctrl.Result{}, nil
}

func (r *PlatformOperatorReconciler) ensureDesiredBundle(ctx context.Context, po *platformv1alpha1.PlatformOperator) (string, error) {
	// check whether we've already sourced a registry+v1 bundle for this
	// platform operator to avoid unnecessarily running the sourcing logic.
	desiredBundle := platformtypes.GetDesiredBundle(po)
	if desiredBundle != "" {
		return desiredBundle, nil
	}

	sourcedBundle, err := r.Sourcer.Source(ctx, po)
	if err != nil {
		return "", err
	}
	return sourcedBundle.Image, nil
}

func (r *PlatformOperatorReconciler) ensureDesiredBundleDeployment(ctx context.Context, po *platformv1alpha1.PlatformOperator) (*rukpakv1alpha1.BundleDeployment, error) {
	object, err := r.Applier.Apply(ctx, po)
	if err != nil {
		return nil, err
	}
	bd, ok := object.(*rukpakv1alpha1.BundleDeployment)
	if !ok {
		panic("failed to type cast client.Object from the Apply method to a BundleDeployment type")
	}
	return bd, nil
}

func (r *PlatformOperatorReconciler) inspectBundleDeploymentStatus(_ context.Context, conditions []metav1.Condition) *metav1.Condition {
	unpacked := meta.FindStatusCondition(conditions, rukpakv1alpha1.TypeHasValidBundle)
	if unpacked == nil {
		return &metav1.Condition{
			Type:    platformtypes.TypeApplied,
			Status:  metav1.ConditionFalse,
			Reason:  platformtypes.ReasonUnpackPending,
			Message: "Waiting for the bundle to be unpacked",
		}
	}
	if unpacked.Status != metav1.ConditionTrue {
		return &metav1.Condition{
			Type:    platformtypes.TypeApplied,
			Status:  metav1.ConditionFalse,
			Reason:  unpacked.Reason,
			Message: unpacked.Message,
		}
	}

	applied := meta.FindStatusCondition(conditions, rukpakv1alpha1.TypeInstalled)
	if applied == nil {
		return &metav1.Condition{
			Type:    platformtypes.TypeApplied,
			Status:  metav1.ConditionFalse,
			Reason:  platformtypes.ReasonUnpackPending,
			Message: "Waiting for the bundle to be unpacked",
		}
	}
	if applied.Status != metav1.ConditionTrue {
		return &metav1.Condition{
			Type:    platformtypes.TypeApplied,
			Status:  metav1.ConditionFalse,
			Reason:  applied.Reason,
			Message: applied.Message,
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlatformOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.PlatformOperator{}).
		Watches(&source.Kind{Type: &operatorsv1alpha1.CatalogSource{}}, handler.EnqueueRequestsFromMapFunc(util.RequeuePlatformOperators(mgr.GetClient()))).
		Watches(&source.Kind{Type: &rukpakv1alpha1.BundleDeployment{}}, handler.EnqueueRequestsFromMapFunc(util.RequeueBundleDeployment(mgr.GetClient()))).
		Complete(r)
}
