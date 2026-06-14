package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloadv1 "github.com/ops-dev/multicloud-workload-deploy/operator/api/v1"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/render"
)

// WorkloadReconciler reconciles a Workload object.
type WorkloadReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=workload.ops.dev,resources=workloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workload.ops.dev,resources=workloads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *WorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var wl workloadv1.Workload
	if err := r.Get(ctx, req.NamespacedName, &wl); err != nil {
		// Not-found → object deleted; child objects are garbage-collected via owner refs.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Snapshot the status base for one MergeFrom patch at the end. We mutate wl.Status in
	// memory and patch exactly once.
	base := wl.DeepCopy()

	objs, err := render.Chart(wl.Name, wl.Namespace, wl.Spec)
	if err != nil {
		meta.SetStatusCondition(&wl.Status.Conditions, cond(&wl, "Ready", metav1.ConditionFalse, "RenderFailed", err.Error()))
		// Surface the status even on the failure path; return the patch error if any so we
		// requeue, else return the render error so controller-runtime requeues with backoff.
		if perr := r.patchStatus(ctx, base, &wl); perr != nil {
			return ctrl.Result{}, perr
		}
		return ctrl.Result{}, fmt.Errorf("render: %w", err)
	}

	for _, obj := range objs {
		if err := setOwnerAndApply(ctx, r, &wl, obj); err != nil {
			meta.SetStatusCondition(&wl.Status.Conditions, cond(&wl, "Ready", metav1.ConditionFalse, "ApplyFailed", err.Error()))
			if perr := r.patchStatus(ctx, base, &wl); perr != nil {
				return ctrl.Result{}, perr
			}
			return ctrl.Result{}, err
		}
	}

	// Canary is requested-but-unsupported in this build. Do not silently ignore it — render
	// RollingUpdate (as the chart always does) and surface a degraded condition. RollingUpdate
	// clears the degraded condition.
	if wl.Spec.RolloutStrategy == workloadv1.RolloutCanary {
		meta.SetStatusCondition(&wl.Status.Conditions, cond(&wl, "RolloutDegraded", metav1.ConditionTrue,
			"CanaryUnsupported", "Canary requested but unsupported in this build → using RollingUpdate"))
	} else {
		meta.SetStatusCondition(&wl.Status.Conditions, cond(&wl, "RolloutDegraded", metav1.ConditionFalse,
			"RollingUpdate", "using RollingUpdate"))
	}

	// Converge ReadyReplicas. Read the live Deployment each reconcile (not once at create). The
	// Owns(&Deployment{}) watch (SetupWithManager) re-triggers Reconcile as the Deployment's
	// status changes, so ReadyReplicas converges to the real value instead of being pinned at 0.
	var dep appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &dep); err == nil {
		wl.Status.ReadyReplicas = dep.Status.ReadyReplicas
	}
	wl.Status.ObservedGeneration = wl.Generation
	meta.SetStatusCondition(&wl.Status.Conditions, cond(&wl, "Ready", metav1.ConditionTrue, "Reconciled", "all child objects applied"))

	// Single status patch at the end. A conflict is returned so controller-runtime requeues.
	if err := r.patchStatus(ctx, base, &wl); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("reconciled workload", "name", wl.Name, "namespace", wl.Namespace, "readyReplicas", wl.Status.ReadyReplicas)
	// Belt-and-suspenders requeue so ReadyReplicas converges even if a Deployment status event
	// is missed; Owns(Deployment) is the primary mechanism.
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// patchStatus applies a single MergeFrom status patch. Conflicts propagate to the caller so the
// request is requeued rather than silently lost.
func (r *WorkloadReconciler) patchStatus(ctx context.Context, base, wl *workloadv1.Workload) error {
	return r.Status().Patch(ctx, wl, client.MergeFrom(base))
}

func cond(wl *workloadv1.Workload, t string, status metav1.ConditionStatus, reason, msg string) metav1.Condition {
	return metav1.Condition{
		Type:               t,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: wl.Generation,
	}
}

func (r *WorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workloadv1.Workload{}).
		Owns(&appsv1.Deployment{}). // re-reconcile on Deployment status changes
		Complete(r)
}
