package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	"github.com/openshift/platform-operators/internal/applier"
	"github.com/openshift/platform-operators/internal/sourcer"
)

var _ = Describe("PlatformOperator Manager", func() {
	var (
		ctx     context.Context
		stopMgr context.CancelFunc
	)
	BeforeEach(func() {
		ctx = context.Background()
	})

	When("the BundleDeployment has been created", func() {
		var (
			r *PlatformOperatorReconciler
			c client.Client
		)
		BeforeEach(func() {
			mgr, err := manager.New(cfg, manager.Options{
				MetricsBindAddress: "0",
				Scheme:             scheme,
			})
			Expect(err).ToNot(HaveOccurred())

			c = mgr.GetClient()

			r = &PlatformOperatorReconciler{
				Client:  c,
				Sourcer: sourcer.NewNoopSourcer(),
				Applier: applier.NewBundleDeploymentHandler(c),
			}
			Expect(r.SetupWithManager(mgr)).To(Succeed())

			stopMgr = StartTestManager(mgr)

		})
		AfterEach(func() {
			stopMgr()
		})

		When("a platformoperator is created", func() {
			var (
				po *platformv1alpha1.PlatformOperator
			)
			BeforeEach(func() {
				po = &platformv1alpha1.PlatformOperator{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "prometheus-operator",
					},
					Spec: platformv1alpha1.PlatformOperatorSpec{
						Package: platformv1alpha1.Package{
							Name: "prometheus-operator",
						},
					},
				}
				Expect(c.Create(ctx, po)).To(BeNil())
			})
			AfterEach(func() {
				Expect(c.Delete(ctx, po)).To(Succeed())
			})
			It("should result in a valid BundleDeployment being created", func() {
				// TODO: how to inject a sourcer.Bundle here using the NoopSourcer?
				res, err := r.reconcile(ctx, po)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(ctrl.Result{}))
			})
		})
	})
})
