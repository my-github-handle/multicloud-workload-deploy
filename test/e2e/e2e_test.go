//go:build e2e

// Package e2e holds real-world tests that require actual infrastructure: a live Kubernetes
// cluster (and, for the cloud paths, a real cloud account). They are guarded by the `e2e` build
// tag so they never compile or run under `mage test` (local unit + envtest). Run them with
// `mage testE2E` against a cluster selected by KUBECONFIG / --kubeconfig, after the operator is
// installed (see test/runbooks/verify-core-on-kind.md).
//
// Unlike the envtest suite in operator/internal/controller (which runs a local apiserver with no
// controller pod), this drives the real operator Deployment end-to-end: apply a Workload, wait
// for the operator to reconcile it, and assert the live child objects.
package e2e

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	workloadv1 "github.com/ops-dev/multicloud-workload-deploy/operator/api/v1"
)

// Config from the environment so the same suite runs on kind or any cloud cluster.
//   - E2E_NAMESPACE   target namespace for the Workload (default: e2e-workload)
//   - E2E_IMAGE       workload image (default: a read-only-friendly public image)
//   - E2E_PORT        workload port (default: 5678)
//   - E2E_RUN_AS_ROOT "true" to relax the hardened security context for images that need root
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func newClient(t *testing.T) client.Client {
	t.Helper()
	cfg, err := config.GetConfig()
	if err != nil {
		t.Fatalf("load kubeconfig (set KUBECONFIG or --kubeconfig): %v", err)
	}
	sch := scheme.Scheme
	if err := workloadv1.AddToScheme(sch); err != nil {
		t.Fatalf("add workload scheme: %v", err)
	}
	c, err := client.New(cfg, client.Options{Scheme: sch})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	return c
}

func eventually(t *testing.T, timeout time.Duration, desc string, fn func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		if last = fn(); last == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timed out waiting for %s: %v", desc, last)
}

// TestWorkloadLifecycle deploys a Workload and asserts the operator reconciles the full child
// set with a Ready status and converged replicas — the real-world counterpart of the envtest
// reconcile specs.
func TestWorkloadLifecycle(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)

	ns := envOr("E2E_NAMESPACE", "e2e-workload")
	name := "e2e-demo"
	port64, err := strconv.ParseInt(envOr("E2E_PORT", "5678"), 10, 32)
	if err != nil {
		t.Fatalf("invalid E2E_PORT: %v", err)
	}
	port := int32(port64)

	// Ensure the namespace exists (idempotent).
	nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	if err := c.Create(ctx, nsObj); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create namespace %s: %v", ns, err)
	}

	wl := &workloadv1.Workload{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: workloadv1.WorkloadSpec{
			Image: envOr("E2E_IMAGE", "hashicorp/http-echo:1.0"),
			Port:  port,
			Autoscale: workloadv1.Autoscale{
				MinReplicas: 2, MaxReplicas: 5, TargetCPUUtilization: 70,
			},
		},
	}
	if os.Getenv("E2E_RUN_AS_ROOT") == "true" {
		runAsNonRoot := false
		roRoot := false
		wl.Spec.PodSecurityContext = &corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot}
		wl.Spec.SecurityContext = &corev1.SecurityContext{ReadOnlyRootFilesystem: &roRoot}
	}

	// Clean any prior run, then create.
	_ = c.Delete(ctx, wl)
	eventually(t, 30*time.Second, "prior Workload deletion", func() error {
		var got workloadv1.Workload
		if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &got); apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("still present")
	})
	if err := c.Create(ctx, wl); err != nil {
		t.Fatalf("create Workload: %v", err)
	}
	t.Cleanup(func() { _ = c.Delete(context.Background(), wl) })

	key := types.NamespacedName{Name: name, Namespace: ns}

	// All child objects appear (operator reconciled them).
	eventually(t, 90*time.Second, "child objects", func() error {
		for _, o := range []client.Object{
			&appsv1.Deployment{}, &corev1.Service{},
			&autoscalingv2.HorizontalPodAutoscaler{}, &policyv1.PodDisruptionBudget{},
		} {
			if err := c.Get(ctx, key, o); err != nil {
				return fmt.Errorf("%T: %w", o, err)
			}
		}
		for _, npName := range []string{name + "-default-deny", name + "-allow"} {
			var np networkingv1.NetworkPolicy
			if err := c.Get(ctx, types.NamespacedName{Name: npName, Namespace: ns}, &np); err != nil {
				return fmt.Errorf("networkpolicy %s: %w", npName, err)
			}
		}
		return nil
	})

	// Workload reports Ready=True.
	eventually(t, 90*time.Second, "Ready=True condition", func() error {
		var got workloadv1.Workload
		if err := c.Get(ctx, key, &got); err != nil {
			return err
		}
		for _, cond := range got.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == metav1.ConditionTrue {
				return nil
			}
		}
		return fmt.Errorf("not ready yet")
	})

	// Pods actually come up: readyReplicas converges to the min.
	eventually(t, 180*time.Second, "readyReplicas >= minReplicas", func() error {
		var got workloadv1.Workload
		if err := c.Get(ctx, key, &got); err != nil {
			return err
		}
		if got.Status.ReadyReplicas >= 2 {
			return nil
		}
		return fmt.Errorf("readyReplicas=%d", got.Status.ReadyReplicas)
	})

	// HPA is wired to the Deployment.
	var hpa autoscalingv2.HorizontalPodAutoscaler
	if err := c.Get(ctx, key, &hpa); err != nil {
		t.Fatalf("get HPA: %v", err)
	}
	if hpa.Spec.ScaleTargetRef.Kind != "Deployment" || hpa.Spec.ScaleTargetRef.Name != name {
		t.Errorf("HPA scaleTargetRef = %s/%s, want Deployment/%s",
			hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name, name)
	}
}
