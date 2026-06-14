package preflight

import (
	"context"
	"fmt"
	"strings"

	authorizationv1 "k8s.io/api/authorization/v1"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// minServerMajor / minServerMinor is the minimum supported Kubernetes control-plane version.
const (
	minServerMajor = 1
	minServerMinor = 26
)

// K8sChecks returns the Kubernetes-facing checks: Stage 4 (infra) and Stage 5 (workload
// readiness). The clients are typed (kubernetes.Interface) and apiextensions (for CRD detection);
// SelfSubjectAccessReviews go through the same typed clientset so fakes are honored. cfg is
// retained for API stability/future use. namespace is the target workload namespace.
func K8sChecks(kc kubernetes.Interface, ac apiextclient.Interface, cfg *rest.Config, namespace string) []Check {
	k := &k8sChecker{kc: kc, ac: ac, cfg: cfg, namespace: namespace}
	return []Check{
		fnCheck{stage: StageKubernetes, name: "kubernetes-infra", fn: k.stage4},
		fnCheck{stage: StageWorkload, name: "workload-readiness", fn: k.stage5},
	}
}

// NoKubeconfigK8sChecks returns a single BLOCKING red Stage-4 check used when no kubeconfig is
// provided on a real invocation. The kubernetes cluster is the primary deploy target; with no
// kubeconfig there is nothing to deploy to, so this must be a red ("cluster not reachable")
// rather than a silent skip that would emit a false green "go".
func NoKubeconfigK8sChecks() []Check {
	return []Check{fnCheck{stage: StageKubernetes, name: "kubernetes-unreachable",
		fn: func(ctx context.Context) []CheckResult {
			return []CheckResult{{ID: "k8s.unreachable", Status: StatusRed,
				Message:     "no kubeconfig provided; the target cluster (deploy target) is not reachable",
				Remediation: "pass --kubeconfig pointing at the target cluster so Stage 4/5 can run"}}
		}}}
}

// fnCheck adapts a method into a Check.
type fnCheck struct {
	stage StageID
	name  string
	fn    func(ctx context.Context) []CheckResult
}

func (c fnCheck) Stage() StageID                        { return c.stage }
func (c fnCheck) Name() string                          { return c.name }
func (c fnCheck) Run(ctx context.Context) []CheckResult { return c.fn(ctx) }

type k8sChecker struct {
	kc        kubernetes.Interface
	ac        apiextclient.Interface
	cfg       *rest.Config
	namespace string
}

// stage4 runs the Stage 4 Kubernetes-infra checks. checkArgoRollouts returns multiple results
// (presence, version, traffic primitive), so it is spread into the slice.
func (k *k8sChecker) stage4(ctx context.Context) []CheckResult {
	out := []CheckResult{
		k.checkNetworkPolicy(ctx),
		k.checkCilium(ctx),
		k.checkMetricsServer(ctx),
		k.checkMinVersion(ctx),
		k.checkPodSecurity(ctx),
		k.checkInstallTier(ctx),
		k.checkWorkloadIdentity(ctx),
	}
	out = append(out, k.checkArgoRollouts(ctx)...)
	return out
}

// stage5 runs the Stage 5 workload-readiness checks.
func (k *k8sChecker) stage5(ctx context.Context) []CheckResult {
	return []CheckResult{
		k.checkNamespaceCreatable(ctx),
		k.checkImagePullable(ctx),
	}
}

// hasAPIGroup reports whether the server serves the given API group (any version). Used for
// built-in API and aggregated-API detection.
func (k *k8sChecker) hasAPIGroup(group string) (bool, error) {
	groups, err := k.kc.Discovery().ServerGroups()
	if err != nil {
		return false, err
	}
	for _, g := range groups.Groups {
		if g.Name == group {
			return true, nil
		}
	}
	return false, nil
}

// hasCRDByGroup reports whether any CRD in the given group exists.
func (k *k8sChecker) hasCRDByGroup(ctx context.Context, group string) (bool, error) {
	list, err := k.ac.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, crd := range list.Items {
		if crd.Spec.Group == group {
			return true, nil
		}
	}
	return false, nil
}

// servedVersionsForKind returns the served CRD version names for the first CRD in the group whose
// Kind matches, plus whether such a CRD was found. Used to inspect the Argo Rollouts CRD's served
// versions for a compatibility check.
func (k *k8sChecker) servedVersionsForKind(ctx context.Context, group, kind string) ([]string, bool, error) {
	list, err := k.ac.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, false, err
	}
	for _, crd := range list.Items {
		if crd.Spec.Group != group || crd.Spec.Names.Kind != kind {
			continue
		}
		var served []string
		for _, v := range crd.Spec.Versions {
			if v.Served {
				served = append(served, v.Name)
			}
		}
		return served, true, nil
	}
	return nil, false, nil
}

func (k *k8sChecker) checkNetworkPolicy(ctx context.Context) CheckResult {
	ok, err := k.hasAPIGroup("networking.k8s.io")
	if err != nil {
		return CheckResult{ID: "k8s.networkpolicy", Status: StatusRed,
			Message:     fmt.Sprintf("could not query API groups: %v", err),
			Remediation: "ensure the cluster is reachable and the deploy identity can list API groups"}
	}
	if !ok {
		return CheckResult{ID: "k8s.networkpolicy", Status: StatusRed,
			Message:     "networking.k8s.io API (NetworkPolicy) is not served by the cluster",
			Remediation: "install a CNI that supports NetworkPolicy; default-deny egress is unenforceable without it"}
	}
	// A served networking.k8s.io API does NOT prove enforcement — flannel, for example, serves
	// the NetworkPolicy API but silently ignores the policies. Whether NetworkPolicy is actually
	// enforced depends on the CNI, which we detect separately (checkCilium) and cannot fully prove
	// from the API surface.
	return CheckResult{ID: "k8s.networkpolicy", Status: StatusGreen,
		Message: "NetworkPolicy API present (enforcement depends on CNI; see k8s.cilium)"}
}

func (k *k8sChecker) checkCilium(ctx context.Context) CheckResult {
	ok, err := k.hasCRDByGroup(ctx, "cilium.io")
	if err != nil {
		return CheckResult{ID: "k8s.cilium", Status: StatusAmber,
			Message:     fmt.Sprintf("could not list CRDs to detect Cilium: %v", err),
			Remediation: "grant list on customresourcedefinitions, or confirm Cilium manually"}
	}
	if !ok {
		return CheckResult{ID: "k8s.cilium", Status: StatusAmber,
			Message:     "Cilium not detected; toFQDNs/Hubble unavailable",
			Remediation: "FQDN egress + L7 flow visibility fall back to the perimeter egress firewall and cloud flow logs (amber gap)"}
	}
	return CheckResult{ID: "k8s.cilium", Status: StatusGreen,
		Message: "Cilium detected (cilium.io CRDs present); toFQDNs/Hubble enhancements available"}
}

func (k *k8sChecker) checkMetricsServer(ctx context.Context) CheckResult {
	ok, err := k.hasAPIGroup("metrics.k8s.io")
	if err != nil {
		return CheckResult{ID: "k8s.metricsserver", Status: StatusAmber,
			Message:     fmt.Sprintf("could not query API groups for metrics-server: %v", err),
			Remediation: "confirm metrics-server is installed; HPA needs metrics.k8s.io"}
	}
	if !ok {
		return CheckResult{ID: "k8s.metricsserver", Status: StatusAmber,
			Message:     "metrics-server not detected (metrics.k8s.io not served); HPA will not scale on CPU",
			Remediation: "install metrics-server so the HorizontalPodAutoscaler can read pod metrics"}
	}
	return CheckResult{ID: "k8s.metricsserver", Status: StatusGreen,
		Message: "metrics-server present (metrics.k8s.io served)"}
}

// minArgoServedVersion is the lowest Argo Rollouts CRD served version we attach to. The Rollout
// CRD has historically served v1alpha1; if a future cluster serves ONLY a higher version we have
// not validated, we flag amber rather than silently attaching.
const minArgoServedVersion = "v1alpha1"

// checkArgoRollouts performs the three-part Stage-4 Argo check: presence, a compatible CRD served
// version, and a traffic-routing primitive (mesh CRDs and/or an IngressClass). The rollout
// outcome (full canary vs replica-weighted vs RollingUpdate fallback) depends on all three, so
// each is its own result. A presence-only check is insufficient.
func (k *k8sChecker) checkArgoRollouts(ctx context.Context) []CheckResult {
	served, found, err := k.servedVersionsForKind(ctx, "argoproj.io", "Rollout")
	if err != nil {
		return []CheckResult{{ID: "k8s.argorollouts", Status: StatusAmber,
			Message:     fmt.Sprintf("could not list CRDs to detect Argo Rollouts: %v", err),
			Remediation: "grant list on customresourcedefinitions, or confirm Argo Rollouts manually"}}
	}
	if !found {
		// Absent → canary unavailable; RollingUpdate fallback. No version/primitive result is
		// meaningful, so emit only the presence result.
		return []CheckResult{{ID: "k8s.argorollouts", Status: StatusAmber,
			Message:     "Argo Rollouts not detected (no argoproj.io Rollout CRD); canary unavailable",
			Remediation: "Canary degrades to Kubernetes-native RollingUpdate (amber gap, not a failure)"}}
	}

	results := []CheckResult{{ID: "k8s.argorollouts", Status: StatusGreen,
		Message: fmt.Sprintf("Argo Rollouts detected (Rollout CRD served versions: %v)", served)}}

	// Version compatibility: we must serve a version we attach to. The served CRD versions are the
	// authoritative, RBAC-cheap signal and are what we attach Rollout objects to.
	compatible := false
	for _, v := range served {
		if v == minArgoServedVersion {
			compatible = true
		}
	}
	if compatible {
		results = append(results, CheckResult{ID: "k8s.argorollouts.version", Status: StatusGreen,
			Message: fmt.Sprintf("compatible Rollout CRD version %q served", minArgoServedVersion)})
	} else {
		results = append(results, CheckResult{ID: "k8s.argorollouts.version", Status: StatusAmber,
			Message:     fmt.Sprintf("Rollout CRD does not serve a version we validated (%q); served=%v", minArgoServedVersion, served),
			Remediation: "align the Argo Rollouts version, or canary attach degrades to RollingUpdate"})
	}

	// Traffic-routing primitive: a mesh CRD (Istio/SMI/Gateway API) and/or an IngressClass enables
	// fine-grained (traffic-%) canary. Without one, canary falls back to replica-weighted.
	results = append(results, k.checkArgoTrafficPrimitive(ctx))
	return results
}

// checkArgoTrafficPrimitive probes for a traffic-routing primitive: a mesh CRD (istio.io /
// split.smi-spec.io / gateway.networking.k8s.io) or an available IngressClass.
func (k *k8sChecker) checkArgoTrafficPrimitive(ctx context.Context) CheckResult {
	for _, g := range []string{"networking.istio.io", "split.smi-spec.io", "gateway.networking.k8s.io"} {
		if ok, err := k.hasCRDByGroup(ctx, g); err == nil && ok {
			return CheckResult{ID: "k8s.argorollouts.traffic", Status: StatusGreen,
				Message: fmt.Sprintf("traffic-routing primitive present (mesh CRD group %q); fine-grained canary available", g)}
		}
	}
	ics, err := k.kc.NetworkingV1().IngressClasses().List(ctx, metav1.ListOptions{})
	if err == nil && len(ics.Items) > 0 {
		return CheckResult{ID: "k8s.argorollouts.traffic", Status: StatusGreen,
			Message: fmt.Sprintf("traffic-routing primitive present (%d IngressClass); ingress-based canary available", len(ics.Items))}
	}
	return CheckResult{ID: "k8s.argorollouts.traffic", Status: StatusAmber,
		Message:     "no mesh CRD or IngressClass detected; canary falls back to replica-weighted (no fine-grained traffic %)",
		Remediation: "install a service mesh or an IngressClass for traffic-percentage canary (amber gap, not a failure)"}
}

// canI runs a SelfSubjectAccessReview for the given verb on a resource. It uses the injected
// clientset directly (so envtest fakes are honored and the path is testable).
func (k *k8sChecker) canI(ctx context.Context, group, resource, verb, namespace string) (bool, error) {
	ssar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:     group,
				Resource:  resource,
				Verb:      verb,
				Namespace: namespace,
			},
		},
	}
	resp, err := k.kc.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, ssar, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	return resp.Status.Allowed, nil
}

// checkMinVersion asserts the control-plane Kubernetes version is at or above the supported floor.
func (k *k8sChecker) checkMinVersion(ctx context.Context) CheckResult {
	info, err := k.kc.Discovery().ServerVersion()
	if err != nil {
		return CheckResult{ID: "k8s.minversion", Status: StatusRed,
			Message:     fmt.Sprintf("could not read server version: %v", err),
			Remediation: "ensure the cluster is reachable so the version can be verified"}
	}
	major, minor, perr := parseMajorMinor(info)
	if perr != nil {
		return CheckResult{ID: "k8s.minversion", Status: StatusAmber,
			Message:     fmt.Sprintf("could not parse server version %q.%q: %v", info.Major, info.Minor, perr),
			Remediation: "verify the cluster version manually meets the supported floor"}
	}
	if major < minServerMajor || (major == minServerMajor && minor < minServerMinor) {
		return CheckResult{ID: "k8s.minversion", Status: StatusRed,
			Message:     fmt.Sprintf("cluster is v%d.%d; minimum supported is v%d.%d", major, minor, minServerMajor, minServerMinor),
			Remediation: fmt.Sprintf("upgrade the cluster to at least v%d.%d", minServerMajor, minServerMinor)}
	}
	return CheckResult{ID: "k8s.minversion", Status: StatusGreen,
		Message: fmt.Sprintf("cluster v%d.%d meets the v%d.%d floor", major, minor, minServerMajor, minServerMinor)}
}

// parseMajorMinor extracts integer major/minor from a version.Info, tolerating the "+" suffix
// some distros append to Minor (e.g. "26+").
func parseMajorMinor(info *version.Info) (int, int, error) {
	clean := func(s string) string { return strings.TrimRight(strings.TrimSpace(s), "+") }
	var major, minor int
	if _, err := fmt.Sscanf(clean(info.Major), "%d", &major); err != nil {
		return 0, 0, err
	}
	if _, err := fmt.Sscanf(clean(info.Minor), "%d", &minor); err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}

// checkPodSecurity detects whether Pod Security admission enforcement is available — the built-in
// PodSecurity admission plugin enforces via the namespace label
// pod-security.kubernetes.io/enforce. We confirm the cluster is recent enough that the plugin is
// on by default (GA in v1.25) and that the namespace-label enforcement mechanism is usable.
func (k *k8sChecker) checkPodSecurity(ctx context.Context) CheckResult {
	info, err := k.kc.Discovery().ServerVersion()
	if err != nil {
		return CheckResult{ID: "k8s.podsecurity", Status: StatusAmber,
			Message:     fmt.Sprintf("could not read server version to confirm PodSecurity admission: %v", err),
			Remediation: "confirm Pod Security admission is enabled (GA since v1.25)"}
	}
	major, minor, perr := parseMajorMinor(info)
	if perr != nil || major < 1 || (major == 1 && minor < 25) {
		return CheckResult{ID: "k8s.podsecurity", Status: StatusAmber,
			Message:     "Pod Security admission may not be GA on this cluster (pre-v1.25 or unparsable version)",
			Remediation: "enable PodSecurity admission, or apply an equivalent admission policy (e.g. Kyverno/Gatekeeper)"}
	}
	// Built-in PSA is GA and on by default; the restricted profile is applied via the namespace
	// enforce label at deploy time (Stage 5 creates the namespace).
	return CheckResult{ID: "k8s.podsecurity", Status: StatusGreen,
		Message: fmt.Sprintf("Pod Security admission available (cluster v%d.%d; restricted profile enforced via namespace label)", major, minor)}
}

// checkWorkloadIdentity is the Kubernetes-side half of the end-to-end Workload Identity binding
// check: the expected workload ServiceAccount either exists carrying the cloud-identity
// annotation, or is creatable. The cloud-side half (SA→cloud-identity resolves) is the cloud
// provider's responsibility. The annotation key is cloud-specific; presence of any recognized WI
// annotation on an existing SA is green.
func (k *k8sChecker) checkWorkloadIdentity(ctx context.Context) CheckResult {
	const saName = "workload" // the workload SA the operator/manifests create
	sa, err := k.kc.CoreV1().ServiceAccounts(k.namespace).Get(ctx, saName, metav1.GetOptions{})
	if err == nil {
		for key := range sa.Annotations {
			if key == "eks.amazonaws.com/role-arn" || // IRSA
				key == "iam.gke.io/gcp-service-account" || // GKE WI
				key == "azure.workload.identity/client-id" { // AKS WI
				return CheckResult{ID: "k8s.workloadidentity", Status: StatusGreen,
					Message: fmt.Sprintf("ServiceAccount %q/%q exists and carries a cloud-identity annotation (%s)", k.namespace, saName, key)}
			}
		}
		return CheckResult{ID: "k8s.workloadidentity", Status: StatusAmber,
			Message:     fmt.Sprintf("ServiceAccount %q/%q exists but carries no recognized Workload Identity annotation", k.namespace, saName),
			Remediation: "annotate the SA for IRSA/GKE WI/AKS WI; the cloud-side trust binding is verified by the cloud provider check"}
	}
	if !apierrors.IsNotFound(err) {
		return CheckResult{ID: "k8s.workloadidentity", Status: StatusAmber,
			Message: fmt.Sprintf("could not read ServiceAccount %q/%q: %v", k.namespace, saName, err)}
	}
	// SA absent — green only if we can create it (and thus set the annotation).
	allowed, ssarErr := k.canI(ctx, "", "serviceaccounts", "create", k.namespace)
	if ssarErr != nil {
		return CheckResult{ID: "k8s.workloadidentity", Status: StatusAmber,
			Message: fmt.Sprintf("SelfSubjectAccessReview for serviceaccount create failed: %v", ssarErr)}
	}
	if !allowed {
		return CheckResult{ID: "k8s.workloadidentity", Status: StatusRed,
			Message:     fmt.Sprintf("workload ServiceAccount %q/%q does not exist and cannot be created", k.namespace, saName),
			Remediation: "grant create on serviceaccounts in the target namespace, or pre-create the annotated SA"}
	}
	return CheckResult{ID: "k8s.workloadidentity", Status: StatusGreen,
		Message: fmt.Sprintf("workload ServiceAccount %q/%q is creatable; the cloud-identity annotation will be applied at deploy and the cloud-side trust binding is verified by the cloud provider check", k.namespace, saName)}
}

// checkInstallTier selects Tier A / Tier B / red:
//   - can create CRDs AND cluster-scoped ClusterRoles → Tier A (operator)  → green
//   - else can create namespaced Deployments          → Tier B (manifests) → amber
//   - else                                            → red, cannot deploy
func (k *k8sChecker) checkInstallTier(ctx context.Context) CheckResult {
	canCRD, err := k.canI(ctx, "apiextensions.k8s.io", "customresourcedefinitions", "create", "")
	if err != nil {
		return CheckResult{ID: "k8s.installtier", Status: StatusRed,
			Message:     fmt.Sprintf("SelfSubjectAccessReview failed: %v", err),
			Remediation: "ensure the deploy identity can self-review its access (authorization.k8s.io)"}
	}
	canClusterRole, err := k.canI(ctx, "rbac.authorization.k8s.io", "clusterroles", "create", "")
	if err != nil {
		return CheckResult{ID: "k8s.installtier", Status: StatusRed,
			Message: fmt.Sprintf("SelfSubjectAccessReview failed: %v", err)}
	}
	canDeploy, err := k.canI(ctx, "apps", "deployments", "create", k.namespace)
	if err != nil {
		return CheckResult{ID: "k8s.installtier", Status: StatusRed,
			Message: fmt.Sprintf("SelfSubjectAccessReview failed: %v", err)}
	}

	switch {
	case canCRD && canClusterRole:
		return CheckResult{ID: "k8s.installtier", Status: StatusGreen,
			Message: "Tier A: can create cluster-scoped CRD + ClusterRole; operator install available"}
	case canDeploy:
		return CheckResult{ID: "k8s.installtier", Status: StatusAmber,
			Message:     "Tier B: namespace-only permissions; operator-less namespaced manifests",
			Remediation: "Workload CR lifecycle (status, drift, canary) unavailable; grant CRD+ClusterRole create for Tier A"}
	default:
		return CheckResult{ID: "k8s.installtier", Status: StatusRed,
			Message:     fmt.Sprintf("cannot create namespaced Deployments in %q; nothing can be deployed", k.namespace),
			Remediation: "grant create on apps/deployments in the target namespace (minimum), or CRD+ClusterRole for Tier A"}
	}
}

// checkNamespaceCreatable confirms the target namespace is free or can be created. If it exists,
// that is fine (green). If it can be created (SSAR allows create on namespaces), green. Otherwise
// red.
func (k *k8sChecker) checkNamespaceCreatable(ctx context.Context) CheckResult {
	_, err := k.kc.CoreV1().Namespaces().Get(ctx, k.namespace, metav1.GetOptions{})
	if err == nil {
		return CheckResult{ID: "workload.namespace", Status: StatusGreen,
			Message: fmt.Sprintf("namespace %q already exists and is usable", k.namespace)}
	}
	if !apierrors.IsNotFound(err) {
		return CheckResult{ID: "workload.namespace", Status: StatusRed,
			Message:     fmt.Sprintf("could not get namespace %q: %v", k.namespace, err),
			Remediation: "ensure the cluster is reachable and the deploy identity can read namespaces"}
	}
	allowed, ssarErr := k.canI(ctx, "", "namespaces", "create", "")
	if ssarErr != nil {
		return CheckResult{ID: "workload.namespace", Status: StatusRed,
			Message: fmt.Sprintf("SelfSubjectAccessReview for namespace create failed: %v", ssarErr)}
	}
	if !allowed {
		return CheckResult{ID: "workload.namespace", Status: StatusRed,
			Message:     fmt.Sprintf("namespace %q does not exist and cannot be created", k.namespace),
			Remediation: "create the namespace ahead of time, or grant create on namespaces"}
	}
	return CheckResult{ID: "workload.namespace", Status: StatusGreen,
		Message: fmt.Sprintf("namespace %q does not exist but is creatable", k.namespace)}
}

// checkImagePullable is best-effort. Without registry credentials the checker cannot perform a
// real pull from a non-cluster context, so it reports an amber "not verified" result rather than
// failing. Real registry probing is deferred to the per-cloud providers, which hold pull creds.
func (k *k8sChecker) checkImagePullable(ctx context.Context) CheckResult {
	return CheckResult{ID: "workload.image", Status: StatusAmber,
		Message:     "image pullability not verified from the preflight context (best-effort)",
		Remediation: "confirm the runtime workload identity can pull the image; verified at first reconcile"}
}
