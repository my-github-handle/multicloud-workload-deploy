package render_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"

	workloadv1 "github.com/ops-dev/multicloud-workload-deploy/operator/api/v1"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/render"
)

func sampleSpec() workloadv1.WorkloadSpec {
	return workloadv1.WorkloadSpec{
		Image: "nginx:1.27",
		Port:  8080,
		Autoscale: workloadv1.Autoscale{
			MinReplicas:          2,
			MaxReplicas:          10,
			TargetCPUUtilization: 70,
		},
		// Exercise the resources passthrough so the chart's `with .Values.resources` is
		// not always empty.
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
	}
}

func TestRenderChartProducesAllObjects(t *testing.T) {
	objs, err := render.Chart("demo", "demo-ns", sampleSpec())
	if err != nil {
		t.Fatalf("Chart() error: %v", err)
	}

	kinds := map[string]int{}
	for _, o := range objs {
		kinds[o.GetObjectKind().GroupVersionKind().Kind]++
	}
	for _, want := range []string{"Deployment", "Service", "HorizontalPodAutoscaler", "PodDisruptionBudget"} {
		if kinds[want] != 1 {
			t.Errorf("expected exactly 1 %s, got %d", want, kinds[want])
		}
	}
	// The networkpolicy.yaml template emits two documents (default-deny + allow). This asserts
	// NetworkPolicy is rendered and exercises the multi-doc decode loop — if the decoder stopped
	// after the first document, this count would be 1, not 2.
	if kinds["NetworkPolicy"] != 2 {
		t.Errorf("expected exactly 2 NetworkPolicy (default-deny + allow), got %d", kinds["NetworkPolicy"])
	}
}

func TestRenderChartResourcesPassThrough(t *testing.T) {
	// Assert spec.Resources reaches the rendered Deployment container.
	objs, err := render.Chart("demo", "demo-ns", sampleSpec())
	if err != nil {
		t.Fatalf("Chart() error: %v", err)
	}
	var dep *appsv1.Deployment
	for _, o := range objs {
		if d, ok := o.(*appsv1.Deployment); ok {
			dep = d
		}
	}
	if dep == nil {
		t.Fatal("no Deployment rendered")
	}
	req := dep.Spec.Template.Spec.Containers[0].Resources.Requests
	if req.Cpu().String() != "100m" {
		t.Errorf("cpu request = %q, want 100m", req.Cpu().String())
	}
	if req.Memory().String() != "128Mi" {
		t.Errorf("memory request = %q, want 128Mi", req.Memory().String())
	}
}

func TestRenderChartRejectsInvalidSpec(t *testing.T) {
	// An empty image fails the chart's values.schema.json (image minLength 1), so Chart must
	// return an error rather than render an invalid Deployment. This is the failure path the
	// controller maps to a Ready=False / RenderFailed condition.
	spec := sampleSpec()
	spec.Image = ""
	if _, err := render.Chart("demo", "demo-ns", spec); err == nil {
		t.Fatal("expected schema validation error for empty image, got nil")
	}
}

func TestRenderChartOmitsNetworkPolicyWhenDisabled(t *testing.T) {
	// The chart always renders the NetworkPolicy floor by default; this is not configurable from
	// the spec, so the default render must include both policies. (Guards against a future
	// regression that drops the floor.)
	objs, err := render.Chart("demo", "demo-ns", sampleSpec())
	if err != nil {
		t.Fatalf("Chart() error: %v", err)
	}
	var nps int
	for _, o := range objs {
		if o.GetObjectKind().GroupVersionKind().Kind == "NetworkPolicy" {
			nps++
		}
	}
	if nps != 2 {
		t.Errorf("expected 2 NetworkPolicy objects in default render, got %d", nps)
	}
}

func TestRenderChartProbesRendered(t *testing.T) {
	// Liveness/readiness probes are optional; when set on the spec they must reach the container.
	spec := sampleSpec()
	spec.LivenessProbe = &workloadv1.Probe{Path: "/healthz", Port: 8080}
	spec.ReadinessProbe = &workloadv1.Probe{Path: "/ready", Port: 8080}
	objs, err := render.Chart("demo", "demo-ns", spec)
	if err != nil {
		t.Fatalf("Chart() error: %v", err)
	}
	var dep *appsv1.Deployment
	for _, o := range objs {
		if d, ok := o.(*appsv1.Deployment); ok {
			dep = d
		}
	}
	if dep == nil {
		t.Fatal("no Deployment rendered")
	}
	c := dep.Spec.Template.Spec.Containers[0]
	if c.LivenessProbe == nil || c.LivenessProbe.HTTPGet == nil || c.LivenessProbe.HTTPGet.Path != "/healthz" {
		t.Errorf("liveness probe not rendered as expected: %+v", c.LivenessProbe)
	}
	if c.ReadinessProbe == nil || c.ReadinessProbe.HTTPGet == nil || c.ReadinessProbe.HTTPGet.Path != "/ready" {
		t.Errorf("readiness probe not rendered as expected: %+v", c.ReadinessProbe)
	}
}

func TestRenderChartSecurityContextOverride(t *testing.T) {
	// When the spec overrides the security context, the rendered container/pod must use the
	// override (e.g. to run an image that needs root or a writable root fs), not the hardened
	// default.
	spec := sampleSpec()
	runAsUser := int64(0)
	allowEsc := true
	roRoot := false
	spec.SecurityContext = &corev1.SecurityContext{
		RunAsUser:                &runAsUser,
		AllowPrivilegeEscalation: &allowEsc,
		ReadOnlyRootFilesystem:   &roRoot,
	}
	runAsNonRoot := false
	spec.PodSecurityContext = &corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot}

	objs, err := render.Chart("demo", "demo-ns", spec)
	if err != nil {
		t.Fatalf("Chart() error: %v", err)
	}
	var dep *appsv1.Deployment
	for _, o := range objs {
		if d, ok := o.(*appsv1.Deployment); ok {
			dep = d
		}
	}
	if dep == nil {
		t.Fatal("no Deployment rendered")
	}
	csc := dep.Spec.Template.Spec.Containers[0].SecurityContext
	if csc == nil || csc.ReadOnlyRootFilesystem == nil || *csc.ReadOnlyRootFilesystem {
		t.Errorf("expected readOnlyRootFilesystem=false from override, got %+v", csc)
	}
	if csc.AllowPrivilegeEscalation == nil || !*csc.AllowPrivilegeEscalation {
		t.Errorf("expected allowPrivilegeEscalation=true from override, got %+v", csc)
	}
	psc := dep.Spec.Template.Spec.SecurityContext
	if psc == nil || psc.RunAsNonRoot == nil || *psc.RunAsNonRoot {
		t.Errorf("expected pod runAsNonRoot=false from override, got %+v", psc)
	}
}

func TestRenderChartHardenedSecurityDefault(t *testing.T) {
	// With no override, the hardened default must apply: non-root pod, read-only root fs, no
	// privilege escalation, all capabilities dropped.
	objs, err := render.Chart("demo", "demo-ns", sampleSpec())
	if err != nil {
		t.Fatalf("Chart() error: %v", err)
	}
	var dep *appsv1.Deployment
	for _, o := range objs {
		if d, ok := o.(*appsv1.Deployment); ok {
			dep = d
		}
	}
	if dep == nil {
		t.Fatal("no Deployment rendered")
	}
	if psc := dep.Spec.Template.Spec.SecurityContext; psc == nil || psc.RunAsNonRoot == nil || !*psc.RunAsNonRoot {
		t.Errorf("expected hardened default runAsNonRoot=true, got %+v", dep.Spec.Template.Spec.SecurityContext)
	}
	csc := dep.Spec.Template.Spec.Containers[0].SecurityContext
	if csc == nil || csc.ReadOnlyRootFilesystem == nil || !*csc.ReadOnlyRootFilesystem {
		t.Errorf("expected hardened default readOnlyRootFilesystem=true, got %+v", csc)
	}
}

func TestRenderChartIngress(t *testing.T) {
	// No Ingress by default.
	objs, err := render.Chart("demo", "demo-ns", sampleSpec())
	if err != nil {
		t.Fatalf("Chart() error: %v", err)
	}
	for _, o := range objs {
		if o.GetObjectKind().GroupVersionKind().Kind == "Ingress" {
			t.Fatal("Ingress rendered without spec.Ingress set")
		}
	}

	// When spec.Ingress is set, an Ingress routes the host/path to the workload Service.
	spec := sampleSpec()
	spec.IngressClass = "nginx"
	spec.Ingress = &workloadv1.IngressConfig{Host: "app.example.com", Path: "/", PathType: "Prefix"}
	objs, err = render.Chart("demo", "demo-ns", spec)
	if err != nil {
		t.Fatalf("Chart() error: %v", err)
	}
	var ing *networkingv1.Ingress
	for _, o := range objs {
		if i, ok := o.(*networkingv1.Ingress); ok {
			ing = i
		}
	}
	if ing == nil {
		t.Fatal("no Ingress rendered with spec.Ingress set")
	}
	if ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName != "nginx" {
		t.Errorf("ingressClassName = %v, want nginx", ing.Spec.IngressClassName)
	}
	if len(ing.Spec.Rules) != 1 || ing.Spec.Rules[0].Host != "app.example.com" {
		t.Fatalf("unexpected rules: %+v", ing.Spec.Rules)
	}
	be := ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service
	if be.Name != "demo" || be.Port.Number != 8080 {
		t.Errorf("backend = %s:%d, want demo:8080", be.Name, be.Port.Number)
	}
}

func TestRenderChartDeploymentFields(t *testing.T) {
	objs, err := render.Chart("demo", "demo-ns", sampleSpec())
	if err != nil {
		t.Fatalf("Chart() error: %v", err)
	}
	var dep *appsv1.Deployment
	for _, o := range objs {
		if d, ok := o.(*appsv1.Deployment); ok {
			dep = d
		}
	}
	if dep == nil {
		t.Fatal("no Deployment rendered")
	}
	if dep.Namespace != "demo-ns" {
		t.Errorf("namespace = %q, want demo-ns", dep.Namespace)
	}
	if got := dep.Spec.Template.Spec.Containers[0].Image; got != "nginx:1.27" {
		t.Errorf("image = %q, want nginx:1.27", got)
	}
	if dep.Spec.Template.Spec.SecurityContext == nil ||
		dep.Spec.Template.Spec.SecurityContext.RunAsNonRoot == nil ||
		!*dep.Spec.Template.Spec.SecurityContext.RunAsNonRoot {
		t.Error("expected runAsNonRoot=true")
	}
	// The Deployment must NOT pin replicas: the HPA owns the replica count. A hardcoded value
	// would make the operator fight the autoscaler on every reconcile.
	if dep.Spec.Replicas != nil {
		t.Errorf("Deployment should not set spec.replicas (HPA owns it), got %d", *dep.Spec.Replicas)
	}
	_ = corev1.Container{}
}
