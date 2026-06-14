//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloadv1 "github.com/ops-dev/multicloud-workload-deploy/operator/api/v1"
)

// TestHPAScalesUpUnderLoad is a real-world test that proves autoscaling end-to-end: it deploys a
// CPU-bound Workload with a low CPU request, drives HTTP load at it, and asserts the HPA scales
// the Deployment above minReplicas. It is slow (HPA reacts on a ~15s sync + stabilization window)
// and requires metrics-server on the cluster, so it is opt-in via E2E_HPA_SCALE=true.
//
//	E2E_HPA_SCALE=true KUBECONFIG=... go test -tags e2e ./test/e2e/ -run HPAScalesUp -v -timeout 15m
func TestHPAScalesUpUnderLoad(t *testing.T) {
	if os.Getenv("E2E_HPA_SCALE") != "true" {
		t.Skip("set E2E_HPA_SCALE=true to run the HPA scale-up test (slow; needs metrics-server)")
	}
	ctx := context.Background()
	c := newClient(t)

	ns := envOr("E2E_NAMESPACE", "e2e-workload")
	name := "hpa-load"
	const minReplicas = int32(1)

	nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	if err := c.Create(ctx, nsObj); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create namespace %s: %v", ns, err)
	}

	// registry.k8s.io/hpa-example is the canonical CPU-burning HPA demo (apache+php). It runs as
	// root and writes to its filesystem, so relax the hardened defaults for it. A low CPU request
	// makes utilization easy to exceed under load.
	wl := &workloadv1.Workload{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: workloadv1.WorkloadSpec{
			Image: envOr("E2E_HPA_IMAGE", "registry.k8s.io/hpa-example"),
			Port:  80,
			Autoscale: workloadv1.Autoscale{
				MinReplicas: minReplicas, MaxReplicas: 5, TargetCPUUtilization: 50,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
			},
			PodSecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: boolp(false)},
			SecurityContext:    &corev1.SecurityContext{ReadOnlyRootFilesystem: boolp(false)},
		},
	}
	_ = c.Delete(ctx, wl)
	time.Sleep(3 * time.Second)
	if err := c.Create(ctx, wl); err != nil {
		t.Fatalf("create Workload: %v", err)
	}
	t.Cleanup(func() {
		_ = c.Delete(context.Background(), wl)
		_ = c.Delete(context.Background(), loadGenerator(ns, name))
	})

	key := types.NamespacedName{Name: name, Namespace: ns}

	// Wait until the workload is serving (min replicas ready) before applying load.
	eventually(t, 180*time.Second, "workload ready at min replicas", func() error {
		var dep appsv1.Deployment
		if err := c.Get(ctx, key, &dep); err != nil {
			return err
		}
		if dep.Status.ReadyReplicas >= minReplicas {
			return nil
		}
		return fmt.Errorf("readyReplicas=%d", dep.Status.ReadyReplicas)
	})

	// Start the load generators: several pods hammering the workload Service in a tight loop.
	lg := loadGenerator(ns, name)
	if err := c.Create(ctx, lg); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create load generator: %v", err)
	}
	t.Logf("load generator running; waiting for the HPA to scale %s above %d replica(s)", name, minReplicas)

	// The HPA should drive the Deployment's desired replicas above the minimum. Poll the
	// Deployment spec (the HPA writes spec.replicas) — generous timeout for metrics + sync windows.
	var maxObserved int32
	eventually(t, 10*time.Minute, "HPA scale-up above minReplicas", func() error {
		var dep appsv1.Deployment
		if err := c.Get(ctx, key, &dep); err != nil {
			return err
		}
		if dep.Spec.Replicas != nil && *dep.Spec.Replicas > maxObserved {
			maxObserved = *dep.Spec.Replicas
		}
		if maxObserved > minReplicas {
			return nil
		}
		return fmt.Errorf("desired replicas still %d", maxObserved)
	})
	t.Logf("HPA scaled %s up to %d replicas under load", name, maxObserved)
}

func boolp(b bool) *bool { return &b }

// loadGenerator returns a Deployment of busybox pods that continuously GET the workload Service,
// generating enough request volume to drive the CPU-bound demo image past its HPA target.
func loadGenerator(ns, target string) *appsv1.Deployment {
	replicas := int32(3)
	labels := map[string]string{"app": "e2e-load-generator", "targets": target}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "load-generator", Namespace: ns, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "load",
						Image: "busybox:1.36",
						Command: []string{"/bin/sh", "-c",
							fmt.Sprintf("while true; do wget -q -O- http://%s.%s:80/ >/dev/null 2>&1; done", target, ns)},
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             boolp(true),
							RunAsUser:                int64p(1000),
							AllowPrivilegeEscalation: boolp(false),
							ReadOnlyRootFilesystem:   boolp(true),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					}},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   boolp(true),
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					},
				},
			},
		},
	}
}

func int64p(i int64) *int64 { return &i }

var _ = client.IgnoreNotFound
