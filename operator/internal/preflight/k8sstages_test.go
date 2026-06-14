package preflight_test

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func requireEnv(t *testing.T) {
	t.Helper()
	if testCfg == nil {
		t.Skip("envtest not available (set KUBEBUILDER_ASSETS via mage test)")
	}
}

func newClients(t *testing.T) (kubernetes.Interface, apiextclient.Interface) {
	t.Helper()
	kc, err := kubernetes.NewForConfig(testCfg)
	if err != nil {
		t.Fatalf("kubernetes client: %v", err)
	}
	ac, err := apiextclient.NewForConfig(testCfg)
	if err != nil {
		t.Fatalf("apiextensions client: %v", err)
	}
	return kc, ac
}

// installCRD applies a minimal CRD (single served version "v1") so detection checks can find it.
func installCRD(t *testing.T, ac apiextclient.Interface, group, plural, kind string) {
	t.Helper()
	installCRDVersion(t, ac, group, plural, kind, "v1")
}

// installCRDVersion applies a minimal CRD with a caller-chosen served version, for the Argo
// version-compatibility fixtures.
func installCRDVersion(t *testing.T, ac apiextclient.Interface, group, plural, kind, ver string) {
	t.Helper()
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: plural + "." + group},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				// singular must be a lowercase DNS-1035 label, distinct from the PascalCase Kind.
				Plural: plural, Kind: kind, Singular: strings.ToLower(kind),
			},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name: ver, Served: true, Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
				},
			}},
		},
	}
	_, err := ac.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), crd, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create CRD %s: %v", crd.Name, err)
	}
}

func TestK8sChecksNoCiliumNoMetricsNoArgo(t *testing.T) {
	requireEnv(t)
	kc, ac := newClients(t)
	checks := preflight.K8sChecks(kc, ac, testCfg, "preflight-test-ns")

	r := preflight.NewRunner(checks)
	report := r.Run(context.Background())

	stage4 := report.Stages[preflight.StageKubernetes]
	byID := map[string]preflight.CheckResult{}
	for _, res := range stage4.Results {
		byID[res.ID] = res
	}

	// NetworkPolicy is a built-in API on envtest → green.
	if byID["k8s.networkpolicy"].Status != preflight.StatusGreen {
		t.Errorf("networkpolicy = %q, want green", byID["k8s.networkpolicy"].Status)
	}
	// No Cilium CRDs installed → amber gap.
	if byID["k8s.cilium"].Status != preflight.StatusAmber {
		t.Errorf("cilium = %q, want amber", byID["k8s.cilium"].Status)
	}
	// metrics-server is not present on bare envtest → amber.
	if byID["k8s.metricsserver"].Status != preflight.StatusAmber {
		t.Errorf("metrics-server = %q, want amber", byID["k8s.metricsserver"].Status)
	}
	// No Argo Rollouts CRD → amber (canary unavailable, RollingUpdate fallback).
	if byID["k8s.argorollouts"].Status != preflight.StatusAmber {
		t.Errorf("argo rollouts = %q, want amber", byID["k8s.argorollouts"].Status)
	}
	// Absent Argo → no version/traffic sub-results emitted (presence-only).
	if _, ok := byID["k8s.argorollouts.version"]; ok {
		t.Error("did not expect k8s.argorollouts.version result when Argo is absent")
	}
	// min-version: envtest's API server is modern → green.
	if byID["k8s.minversion"].Status != preflight.StatusGreen {
		t.Errorf("min version = %q, want green (envtest is modern)", byID["k8s.minversion"].Status)
	}
	// PodSecurity admission GA on a modern cluster → green.
	if byID["k8s.podsecurity"].Status != preflight.StatusGreen {
		t.Errorf("podsecurity = %q, want green", byID["k8s.podsecurity"].Status)
	}
	// Workload Identity: SA absent but creatable on envtest (full access) → green.
	if byID["k8s.workloadidentity"].Status != preflight.StatusGreen {
		t.Errorf("workloadidentity = %q, want green (SA creatable)", byID["k8s.workloadidentity"].Status)
	}
	// envtest grants the test client full access → Tier A (green).
	if byID["k8s.installtier"].Status != preflight.StatusGreen {
		t.Errorf("install tier = %q, want green (Tier A)", byID["k8s.installtier"].Status)
	}
	if byID["k8s.installtier"].Message == "" {
		t.Error("install tier message empty; expected it to name the selected tier")
	}
}

func TestK8sChecksDetectsCiliumAndArgo(t *testing.T) {
	requireEnv(t)
	kc, ac := newClients(t)
	installCRD(t, ac, "cilium.io", "ciliumnetworkpolicies", "CiliumNetworkPolicy")
	// present-COMPATIBLE: served version matches the floor (v1alpha1).
	installCRDVersion(t, ac, "argoproj.io", "rollouts", "Rollout", "v1alpha1")
	// a traffic-routing primitive: an IngressClass.
	if _, err := kc.NetworkingV1().IngressClasses().Create(context.Background(),
		newIngressClass("nginx"), metav1.CreateOptions{}); err != nil {
		t.Fatalf("create IngressClass: %v", err)
	}

	checks := preflight.K8sChecks(kc, ac, testCfg, "preflight-test-ns-2")
	r := preflight.NewRunner(checks)
	report := r.Run(context.Background())

	stage4 := report.Stages[preflight.StageKubernetes]
	byID := map[string]preflight.CheckResult{}
	for _, res := range stage4.Results {
		byID[res.ID] = res
	}
	if byID["k8s.cilium"].Status != preflight.StatusGreen {
		t.Errorf("cilium = %q, want green (CRD installed)", byID["k8s.cilium"].Status)
	}
	if byID["k8s.argorollouts"].Status != preflight.StatusGreen {
		t.Errorf("argo rollouts = %q, want green (CRD installed)", byID["k8s.argorollouts"].Status)
	}
	// presence is not enough — version + traffic-routing sub-results emitted.
	if byID["k8s.argorollouts.version"].Status != preflight.StatusGreen {
		t.Errorf("argo version = %q, want green (compatible v1alpha1 served)", byID["k8s.argorollouts.version"].Status)
	}
	if byID["k8s.argorollouts.traffic"].Status != preflight.StatusGreen {
		t.Errorf("argo traffic primitive = %q, want green (IngressClass present)", byID["k8s.argorollouts.traffic"].Status)
	}
}

// newIngressClass is a tiny helper for the traffic-primitive fixture.
func newIngressClass(name string) *networkingv1.IngressClass {
	return &networkingv1.IngressClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       networkingv1.IngressClassSpec{Controller: "example.com/ingress-controller"},
	}
}

// TestArgoRolloutsVersionAndTrafficFixtures exercises absent, present-incompatible-version, and
// present-no-traffic-primitive. Each runs in its own namespace; CRDs are cluster-scoped, so the
// argoproj.io Rollout CRD is (re)created per subtest with the relevant version.
func TestArgoRolloutsVersionAndTrafficFixtures(t *testing.T) {
	requireEnv(t)
	kc, ac := newClients(t)
	ctx := context.Background()

	// deleteRolloutCRD removes the cluster-scoped Rollout CRD and waits for it to be fully gone.
	// CRD deletion is asynchronous (finalizers), and these subtests share one envtest, so a
	// recreate that races a pending delete fails with "object is being deleted" and a detection
	// check can briefly still see the old CRD. Block until it is actually absent.
	deleteRolloutCRD := func() {
		t.Helper()
		_ = ac.ApiextensionsV1().CustomResourceDefinitions().
			Delete(ctx, "rollouts.argoproj.io", metav1.DeleteOptions{})
		for i := 0; i < 100; i++ {
			_, err := ac.ApiextensionsV1().CustomResourceDefinitions().
				Get(ctx, "rollouts.argoproj.io", metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("timed out waiting for rollouts.argoproj.io CRD to delete")
	}

	stage4ByID := func(ns string) map[string]preflight.CheckResult {
		report := preflight.NewRunner(preflight.K8sChecks(kc, ac, testCfg, ns)).Run(ctx)
		byID := map[string]preflight.CheckResult{}
		for _, res := range report.Stages[preflight.StageKubernetes].Results {
			byID[res.ID] = res
		}
		return byID
	}

	t.Run("absent", func(t *testing.T) {
		deleteRolloutCRD()
		byID := stage4ByID("argo-absent-ns")
		if byID["k8s.argorollouts"].Status != preflight.StatusAmber {
			t.Errorf("presence = %q, want amber (absent)", byID["k8s.argorollouts"].Status)
		}
		if _, ok := byID["k8s.argorollouts.version"]; ok {
			t.Error("no version sub-result expected when absent")
		}
	})

	t.Run("present-incompatible-version", func(t *testing.T) {
		deleteRolloutCRD()
		installCRDVersion(t, ac, "argoproj.io", "rollouts", "Rollout", "v1beta99")
		byID := stage4ByID("argo-incompat-ns")
		if byID["k8s.argorollouts.version"].Status != preflight.StatusAmber {
			t.Errorf("version = %q, want amber (incompatible served version)", byID["k8s.argorollouts.version"].Status)
		}
	})

	t.Run("present-no-traffic-primitive", func(t *testing.T) {
		// The traffic primitive is detected from cluster-scoped IngressClasses (and mesh CRDs),
		// which persist across tests in this shared envtest. Remove any IngressClass another test
		// created so this subtest genuinely exercises the no-primitive (amber) path.
		ics, _ := kc.NetworkingV1().IngressClasses().List(ctx, metav1.ListOptions{})
		for _, ic := range ics.Items {
			_ = kc.NetworkingV1().IngressClasses().Delete(ctx, ic.Name, metav1.DeleteOptions{})
		}
		deleteRolloutCRD()
		installCRDVersion(t, ac, "argoproj.io", "rollouts", "Rollout", "v1alpha1")
		byID := stage4ByID("argo-notraffic-ns")
		if byID["k8s.argorollouts.version"].Status != preflight.StatusGreen {
			t.Errorf("version = %q, want green (compatible)", byID["k8s.argorollouts.version"].Status)
		}
		if byID["k8s.argorollouts.traffic"].Status != preflight.StatusAmber {
			t.Errorf("traffic = %q, want amber (no mesh/IngressClass)", byID["k8s.argorollouts.traffic"].Status)
		}
	})
}

func TestK8sStage5NamespaceCreatable(t *testing.T) {
	requireEnv(t)
	kc, ac := newClients(t)
	checks := preflight.K8sChecks(kc, ac, testCfg, "preflight-fresh-ns")
	r := preflight.NewRunner(checks)
	report := r.Run(context.Background())

	stage5 := report.Stages[preflight.StageWorkload]
	var found bool
	for _, res := range stage5.Results {
		if res.ID == "workload.namespace" {
			found = true
			if res.Status != preflight.StatusGreen {
				t.Errorf("namespace check = %q, want green", res.Status)
			}
		}
	}
	if !found {
		t.Error("no workload.namespace check result in stage 5")
	}
}

// TestK8sWorkloadIdentityAnnotatedSA covers the K8s-side WI binding check when the workload
// ServiceAccount already exists with a cloud-identity annotation → green; and when it exists
// WITHOUT one → amber.
func TestK8sWorkloadIdentityAnnotatedSA(t *testing.T) {
	requireEnv(t)
	kc, ac := newClients(t)
	ctx := context.Background()

	mkNS := func(ns string) {
		_, _ = kc.CoreV1().Namespaces().Create(ctx,
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}, metav1.CreateOptions{})
	}
	mkSA := func(ns string, ann map[string]string) {
		_, err := kc.CoreV1().ServiceAccounts(ns).Create(ctx,
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "workload", Namespace: ns, Annotations: ann}},
			metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("create SA: %v", err)
		}
	}
	wiID := func(ns string) preflight.CheckResult {
		report := preflight.NewRunner(preflight.K8sChecks(kc, ac, testCfg, ns)).Run(ctx)
		for _, res := range report.Stages[preflight.StageKubernetes].Results {
			if res.ID == "k8s.workloadidentity" {
				return res
			}
		}
		t.Fatal("no k8s.workloadidentity result")
		return preflight.CheckResult{}
	}

	mkNS("wi-annotated")
	mkSA("wi-annotated", map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::123:role/x"})
	if got := wiID("wi-annotated"); got.Status != preflight.StatusGreen {
		t.Errorf("annotated SA = %q, want green", got.Status)
	}

	mkNS("wi-bare")
	mkSA("wi-bare", nil)
	if got := wiID("wi-bare"); got.Status != preflight.StatusAmber {
		t.Errorf("bare SA = %q, want amber (no WI annotation)", got.Status)
	}
}

// TestK8sMetricsServerGreenViaAPIService closes the metrics-server green-path gap by registering
// an APIService for metrics.k8s.io (the same way metrics-server advertises itself), which makes
// the group appear in discovery WITHOUT a backing pod. It self-skips if the kube-aggregator
// clientset or the apiregistration group is unavailable on this envtest.
func TestK8sMetricsServerGreenViaAPIService(t *testing.T) {
	requireEnv(t)
	kc, ac := newClients(t)
	ctx := context.Background()

	aggClient, err := aggregatorclient.NewForConfig(testCfg)
	if err != nil {
		t.Skipf("kube-aggregator client unavailable: %v", err)
	}
	apisvc := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: "v1beta1.metrics.k8s.io"},
		Spec: apiregistrationv1.APIServiceSpec{
			Group: "metrics.k8s.io", Version: "v1beta1",
			GroupPriorityMinimum: 100, VersionPriority: 100,
			Service: &apiregistrationv1.ServiceReference{Namespace: "kube-system", Name: "metrics-server"},
		},
	}
	if _, err := aggClient.ApiregistrationV1().APIServices().Create(ctx, apisvc, metav1.CreateOptions{}); err != nil {
		t.Skipf("could not register metrics.k8s.io APIService on this envtest: %v", err)
	}
	t.Cleanup(func() {
		_ = aggClient.ApiregistrationV1().APIServices().Delete(ctx, apisvc.Name, metav1.DeleteOptions{})
	})

	// Discovery publication is asynchronous: the metrics.k8s.io group does not appear in
	// ServerGroups the instant the APIService is created. Poll until the check goes green (or
	// time out) so this does not flake.
	metricsResult := func() preflight.CheckResult {
		report := preflight.NewRunner(preflight.K8sChecks(kc, ac, testCfg, "metrics-green-ns")).Run(ctx)
		for _, res := range report.Stages[preflight.StageKubernetes].Results {
			if res.ID == "k8s.metricsserver" {
				return res
			}
		}
		t.Fatal("no k8s.metricsserver result")
		return preflight.CheckResult{}
	}
	var last preflight.CheckResult
	for i := 0; i < 100; i++ {
		last = metricsResult()
		if last.Status == preflight.StatusGreen {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("metrics-server = %q, want green (metrics.k8s.io APIService registered)", last.Status)
}
