package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloadv1 "github.com/ops-dev/multicloud-workload-deploy/operator/api/v1"
)

// setOwnerAndApply sets the Workload as controller-owner and creates-or-updates the object,
// preserving immutable server-populated fields so an update never strips e.g. a Service's
// clusterIP or a Deployment's selector.
func setOwnerAndApply(ctx context.Context, r *WorkloadReconciler, wl *workloadv1.Workload, desired client.Object) error {
	// The object handed to CreateOrUpdate is the in-cluster target; the mutate fn copies the
	// desired spec onto it each call. On create it is empty; on update it carries existing
	// immutable fields we must not clobber.
	target := desired.DeepCopyObject().(client.Object)
	target.SetName(desired.GetName())
	target.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, target, func() error {
		// Always (re)assert ownership so GC + Owns() work.
		if err := controllerutil.SetControllerReference(wl, target, r.Scheme); err != nil {
			return err
		}
		// Reconcile desired labels/annotations onto the live object so governance labels do not
		// drift on update. target is loaded from live state, so without this a relabel of the
		// chart would never reach existing objects.
		reconcileMetadata(desired, target)
		// Copy desired spec/data onto target while preserving immutable server-owned fields.
		switch d := desired.(type) {
		case *corev1.Service:
			t := target.(*corev1.Service)
			clusterIP, clusterIPs := t.Spec.ClusterIP, t.Spec.ClusterIPs // server-assigned, immutable
			d.Spec.DeepCopyInto(&t.Spec)
			if clusterIP != "" {
				t.Spec.ClusterIP, t.Spec.ClusterIPs = clusterIP, clusterIPs
			}
		case *appsv1.Deployment:
			t := target.(*appsv1.Deployment)
			if t.Spec.Selector == nil { // selector is immutable once set; keep existing
				t.Spec.Selector = d.Spec.Selector
			}
			selector := t.Spec.Selector
			// The HPA owns spec.replicas. The desired Deployment carries no replicas (nil), so
			// preserve the live value rather than reset it — otherwise each reconcile would fight
			// the autoscaler back down to the default.
			replicas := t.Spec.Replicas
			d.Spec.DeepCopyInto(&t.Spec)
			t.Spec.Selector = selector
			t.Spec.Replicas = replicas
		case *autoscalingv2.HorizontalPodAutoscaler:
			d.Spec.DeepCopyInto(&target.(*autoscalingv2.HorizontalPodAutoscaler).Spec)
		case *policyv1.PodDisruptionBudget:
			d.Spec.DeepCopyInto(&target.(*policyv1.PodDisruptionBudget).Spec)
		case *networkingv1.NetworkPolicy:
			d.Spec.DeepCopyInto(&target.(*networkingv1.NetworkPolicy).Spec)
		case *networkingv1.Ingress:
			d.Spec.DeepCopyInto(&target.(*networkingv1.Ingress).Spec)
		default:
			return fmt.Errorf("apply: unsupported object kind %T", desired)
		}
		return nil
	})
	return err
}

// reconcileMetadata copies the desired labels and annotations onto the live target, merging
// (desired wins per key) rather than replacing, so operator/server-added entries on the live
// object survive while governance labels stay in sync with the chart.
func reconcileMetadata(desired, target client.Object) {
	target.SetLabels(mergeStringMaps(target.GetLabels(), desired.GetLabels()))
	target.SetAnnotations(mergeStringMaps(target.GetAnnotations(), desired.GetAnnotations()))
}

func mergeStringMaps(existing, desired map[string]string) map[string]string {
	if existing == nil && desired == nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range desired {
		out[k] = v
	}
	return out
}
