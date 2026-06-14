package controller_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloadv1 "github.com/ops-dev/multicloud-workload-deploy/operator/api/v1"
)

func baseSpec() workloadv1.WorkloadSpec {
	return workloadv1.WorkloadSpec{
		Image:     "nginx:1.27",
		Port:      8080,
		Autoscale: workloadv1.Autoscale{MinReplicas: 2, MaxReplicas: 10, TargetCPUUtilization: 70},
	}
}

var _ = Describe("Workload controller", func() {
	const ns = "default"

	It("creates Deployment, Service, HPA, PDB, and NetworkPolicies owned by the Workload", func() {
		wl := &workloadv1.Workload{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: ns},
			Spec:       baseSpec(),
		}
		Expect(k8sClient.Create(ctx, wl)).To(Succeed())

		key := types.NamespacedName{Name: "demo", Namespace: ns}

		// All child objects are applied in a single reconcile, but in map-iteration order, so wait
		// on the full set rather than a single object before asserting fields.
		var (
			dep     appsv1.Deployment
			svc     corev1.Service
			hpa     autoscalingv2.HorizontalPodAutoscaler
			pdb     policyv1.PodDisruptionBudget
			denyNP  networkingv1.NetworkPolicy
			allowNP networkingv1.NetworkPolicy
		)
		Eventually(func() error {
			if err := k8sClient.Get(ctx, key, &dep); err != nil {
				return err
			}
			if err := k8sClient.Get(ctx, key, &svc); err != nil {
				return err
			}
			if err := k8sClient.Get(ctx, key, &hpa); err != nil {
				return err
			}
			if err := k8sClient.Get(ctx, key, &pdb); err != nil {
				return err
			}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "demo-default-deny", Namespace: ns}, &denyNP); err != nil {
				return err
			}
			return k8sClient.Get(ctx, types.NamespacedName{Name: "demo-allow", Namespace: ns}, &allowNP)
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("nginx:1.27"))
		Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)))
		Expect(*hpa.Spec.MinReplicas).To(Equal(int32(2)))
		Expect(pdb.Spec.MinAvailable).NotTo(BeNil())

		// Every child object must be owned by the Workload (controller owner-ref) so deletion
		// garbage-collects them and the Owns() watch re-reconciles on their changes.
		for _, o := range []client.Object{&dep, &svc, &hpa, &pdb, &denyNP, &allowNP} {
			refs := o.GetOwnerReferences()
			Expect(refs).To(HaveLen(1), "object %T should have exactly one owner ref", o)
			Expect(refs[0].Kind).To(Equal("Workload"))
			Expect(refs[0].Name).To(Equal("demo"))
			Expect(refs[0].Controller).NotTo(BeNil())
			Expect(*refs[0].Controller).To(BeTrue(), "owner ref must be a controller ref")
			Expect(refs[0].BlockOwnerDeletion).NotTo(BeNil())

			// Every child object must carry the governance label set so the workload's resources
			// can be identified and operated on as one unit.
			labels := o.GetLabels()
			Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/name", "demo"))
			Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/instance", "demo"))
			Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", "demo"))
			Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "workload-operator"))
		}
	})

	It("sets a Ready condition on the Workload status", func() {
		key := types.NamespacedName{Name: "demo", Namespace: ns}
		Eventually(func() bool {
			var wl workloadv1.Workload
			if err := k8sClient.Get(ctx, key, &wl); err != nil {
				return false
			}
			return apimeta.IsStatusConditionTrue(wl.Status.Conditions, "Ready")
		}, 10*time.Second, 250*time.Millisecond).Should(BeTrue())
	})

	It("converges ReadyReplicas from the Deployment status", func() {
		// Simulate the Deployment becoming ready; the Owns(Deployment) watch + RequeueAfter must
		// drive the Workload's status.readyReplicas to reflect it (instead of being stuck at 0).
		key := types.NamespacedName{Name: "demo", Namespace: ns}
		var dep appsv1.Deployment
		Expect(k8sClient.Get(ctx, key, &dep)).To(Succeed())
		dep.Status.ReadyReplicas = 2
		dep.Status.Replicas = 2
		Expect(k8sClient.Status().Update(ctx, &dep)).To(Succeed())

		Eventually(func() int32 {
			var wl workloadv1.Workload
			if err := k8sClient.Get(ctx, key, &wl); err != nil {
				return -1
			}
			return wl.Status.ReadyReplicas
		}, 10*time.Second, 250*time.Millisecond).Should(Equal(int32(2)))
	})

	It("updates an existing Workload without stripping the Service clusterIP", func() {
		key := types.NamespacedName{Name: "demo", Namespace: ns}
		var svc corev1.Service
		Expect(k8sClient.Get(ctx, key, &svc)).To(Succeed())
		assignedClusterIP := svc.Spec.ClusterIP
		Expect(assignedClusterIP).NotTo(BeEmpty())

		// Mutate the Workload → triggers the update-existing (requeue) reconcile branch.
		var wl workloadv1.Workload
		Expect(k8sClient.Get(ctx, key, &wl)).To(Succeed())
		wl.Spec.Autoscale.MaxReplicas = 20
		Expect(k8sClient.Update(ctx, &wl)).To(Succeed())

		Eventually(func() int32 {
			var hpa autoscalingv2.HorizontalPodAutoscaler
			if err := k8sClient.Get(ctx, key, &hpa); err != nil {
				return -1
			}
			return hpa.Spec.MaxReplicas
		}, 10*time.Second, 250*time.Millisecond).Should(Equal(int32(20)))

		// The Service's server-assigned clusterIP must survive the update.
		Expect(k8sClient.Get(ctx, key, &svc)).To(Succeed())
		Expect(svc.Spec.ClusterIP).To(Equal(assignedClusterIP))
	})

	It("reports Canary as a degraded condition, not silently", func() {
		wl := &workloadv1.Workload{
			ObjectMeta: metav1.ObjectMeta{Name: "canary-demo", Namespace: ns},
			Spec: func() workloadv1.WorkloadSpec {
				s := baseSpec()
				s.RolloutStrategy = workloadv1.RolloutCanary
				return s
			}(),
		}
		Expect(k8sClient.Create(ctx, wl)).To(Succeed())
		key := types.NamespacedName{Name: "canary-demo", Namespace: ns}

		Eventually(func() bool {
			var got workloadv1.Workload
			if err := k8sClient.Get(ctx, key, &got); err != nil {
				return false
			}
			c := apimeta.FindStatusCondition(got.Status.Conditions, "RolloutDegraded")
			return c != nil && c.Status == metav1.ConditionTrue && c.Reason == "CanaryUnsupported"
		}, 10*time.Second, 250*time.Millisecond).Should(BeTrue())

		// The Deployment is still rendered with RollingUpdate — Canary is reported AND falls back.
		var dep appsv1.Deployment
		Expect(k8sClient.Get(ctx, key, &dep)).To(Succeed())
		Expect(dep.Spec.Strategy.Type).To(Equal(appsv1.RollingUpdateDeploymentStrategyType))
	})

	It("creates an HPA wired to the Deployment with the spec's autoscale bounds", func() {
		wl := &workloadv1.Workload{
			ObjectMeta: metav1.ObjectMeta{Name: "hpa-demo", Namespace: ns},
			Spec: func() workloadv1.WorkloadSpec {
				s := baseSpec()
				s.Autoscale = workloadv1.Autoscale{MinReplicas: 3, MaxReplicas: 12, TargetCPUUtilization: 65}
				return s
			}(),
		}
		Expect(k8sClient.Create(ctx, wl)).To(Succeed())
		key := types.NamespacedName{Name: "hpa-demo", Namespace: ns}

		var hpa autoscalingv2.HorizontalPodAutoscaler
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &hpa)
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		// Bounds come straight from the spec.
		Expect(*hpa.Spec.MinReplicas).To(Equal(int32(3)))
		Expect(hpa.Spec.MaxReplicas).To(Equal(int32(12)))

		// scaleTargetRef must point at the workload's Deployment, else the HPA scales nothing.
		Expect(hpa.Spec.ScaleTargetRef.Kind).To(Equal("Deployment"))
		Expect(hpa.Spec.ScaleTargetRef.Name).To(Equal("hpa-demo"))
		Expect(hpa.Spec.ScaleTargetRef.APIVersion).To(Equal("apps/v1"))

		// CPU utilization metric reflects the spec target.
		Expect(hpa.Spec.Metrics).To(HaveLen(1))
		Expect(hpa.Spec.Metrics[0].Type).To(Equal(autoscalingv2.ResourceMetricSourceType))
		Expect(hpa.Spec.Metrics[0].Resource.Name).To(Equal(corev1.ResourceCPU))
		Expect(*hpa.Spec.Metrics[0].Resource.Target.AverageUtilization).To(Equal(int32(65)))

		// Owned by the Workload so it is garbage-collected with the parent.
		Expect(hpa.OwnerReferences).To(HaveLen(1))
		Expect(hpa.OwnerReferences[0].Kind).To(Equal("Workload"))
	})

	It("propagates autoscale changes to the existing HPA", func() {
		key := types.NamespacedName{Name: "hpa-demo", Namespace: ns}
		var wl workloadv1.Workload
		Expect(k8sClient.Get(ctx, key, &wl)).To(Succeed())
		wl.Spec.Autoscale.MaxReplicas = 20
		wl.Spec.Autoscale.TargetCPUUtilization = 80
		Expect(k8sClient.Update(ctx, &wl)).To(Succeed())

		Eventually(func() int32 {
			var hpa autoscalingv2.HorizontalPodAutoscaler
			if err := k8sClient.Get(ctx, key, &hpa); err != nil {
				return -1
			}
			return hpa.Spec.MaxReplicas
		}, 10*time.Second, 250*time.Millisecond).Should(Equal(int32(20)))

		var hpa autoscalingv2.HorizontalPodAutoscaler
		Expect(k8sClient.Get(ctx, key, &hpa)).To(Succeed())
		Expect(*hpa.Spec.Metrics[0].Resource.Target.AverageUtilization).To(Equal(int32(80)))
	})

	It("creates an Ingress when spec.ingress is set, owned and labeled", func() {
		wl := &workloadv1.Workload{
			ObjectMeta: metav1.ObjectMeta{Name: "ing-demo", Namespace: ns},
			Spec: func() workloadv1.WorkloadSpec {
				s := baseSpec()
				s.IngressClass = "nginx"
				s.Ingress = &workloadv1.IngressConfig{Host: "app.example.com", Path: "/", PathType: "Prefix"}
				return s
			}(),
		}
		Expect(k8sClient.Create(ctx, wl)).To(Succeed())
		key := types.NamespacedName{Name: "ing-demo", Namespace: ns}

		var ing networkingv1.Ingress
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &ing)
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(ing.Spec.IngressClassName).NotTo(BeNil())
		Expect(*ing.Spec.IngressClassName).To(Equal("nginx"))
		Expect(ing.Spec.Rules[0].Host).To(Equal("app.example.com"))
		Expect(ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name).To(Equal("ing-demo"))
		Expect(ing.OwnerReferences).To(HaveLen(1))
		Expect(ing.OwnerReferences[0].Kind).To(Equal("Workload"))
		Expect(ing.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "workload-operator"))
	})

	It("rejects a Workload whose autoscale maxReplicas < minReplicas (CEL validation)", func() {
		wl := &workloadv1.Workload{
			ObjectMeta: metav1.ObjectMeta{Name: "bad-autoscale", Namespace: ns},
			Spec: func() workloadv1.WorkloadSpec {
				s := baseSpec()
				s.Autoscale = workloadv1.Autoscale{MinReplicas: 5, MaxReplicas: 2, TargetCPUUtilization: 70}
				return s
			}(),
		}
		err := k8sClient.Create(ctx, wl)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("maxReplicas must be greater than or equal to minReplicas"))
	})

	It("ignores not-found after the Workload is deleted", func() {
		wl := &workloadv1.Workload{
			ObjectMeta: metav1.ObjectMeta{Name: "ephemeral", Namespace: ns},
			Spec:       baseSpec(),
		}
		Expect(k8sClient.Create(ctx, wl)).To(Succeed())
		Expect(k8sClient.Delete(ctx, wl)).To(Succeed())
		// Reconcile must not error or panic after deletion (owner-ref GC removes children).
		Eventually(func() bool {
			var got workloadv1.Workload
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "ephemeral", Namespace: ns}, &got)
			return err != nil // NotFound
		}, 10*time.Second, 250*time.Millisecond).Should(BeTrue())
	})
})
