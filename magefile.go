//go:build mage

// Magefile drives build, codegen, test, and packaging for the repo-root Go module.
// Run "mage -l" to list targets. The single Go module is rooted here, so every command runs
// from the repo root — the embedded chart resolves and the whole module is compiled and covered.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Tool + asset versions, aligned to controller-runtime v0.21 / k8s 0.33.
const (
	controllerToolsVersion = "v0.18.0"
	// setup-envtest release-0.19+ fetches binaries from the GitHub-hosted envtest-releases.yaml
	// index; release-0.18 used the retired GCS bucket, which now 401s for every version.
	envtestVersion    = "release-0.24"
	envtestK8sVersion = "1.33.0"
	coverageMin       = 80.0
	operatorImage     = "ghcr.io/ops-dev/workload-operator:dev"
)

var (
	localBin      = mustAbs("operator/bin")
	controllerGen = filepath.Join(localBin, "controller-gen")
	setupEnvtest  = filepath.Join(localBin, "setup-envtest")
	coverProfile  = "operator/cover.out"
	// coverPkg scopes the coverage gate to the hand-written operator logic (reconcile loop +
	// chart render), excluding generated deepcopy and the manager main() (whose
	// GetConfigOrDie/Start cannot run without a real apiserver).
	coverPkg = "github.com/ops-dev/multicloud-workload-deploy/operator/internal/controller," +
		"github.com/ops-dev/multicloud-workload-deploy/operator/internal/render," +
		"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight," +
		"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/fake," +
		"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/aws," +
		"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/gcp," +
		"github.com/ops-dev/multicloud-workload-deploy/operator/cmd/preflight"
)

// Default runs the full verification suite.
var Default = Verify

// Verify runs the operator tests (with coverage gate), lints both charts, and builds the
// preflight binary.
func Verify() {
	mg.SerialDeps(Test, LintCharts, PreflightBuild)
}

// PreflightBuild compiles the preflight checker binary.
func PreflightBuild() error {
	return sh.RunV("go", "build", "-o", "operator/bin/preflight", "./operator/cmd/preflight")
}

// Generate writes the deepcopy methods for the API types.
func Generate() error {
	mg.Deps(installControllerGen)
	return sh.RunV(controllerGen, `object:headerFile=operator/hack/boilerplate.go.txt`, `paths=./operator/api/...`)
}

// Manifests generates the CRD YAML from the API types.
func Manifests() error {
	mg.Deps(installControllerGen)
	return sh.RunV(controllerGen, "crd", `paths=./operator/api/...`, `output:crd:artifacts:config=operator/config/crd`)
}

// SyncCRD regenerates the CRD and copies it into the operator install chart so the two never drift.
func SyncCRD() error {
	mg.Deps(Manifests)
	return sh.Copy(
		"charts/workload-operator/crds/workload.ops.dev_workloads.yaml",
		"operator/config/crd/workload.ops.dev_workloads.yaml",
	)
}

// Build compiles the manager binary.
func Build() error {
	return sh.RunV("go", "build", "-o", "operator/bin/manager", "./operator/cmd")
}

// Test runs the LOCAL suite — pure unit tests plus envtest (a local apiserver, no cluster or
// cloud) — across the module and enforces the coverage gate on the operator logic packages.
// Real-world tests that need live infrastructure live in test/e2e (see TestE2E).
func Test() error {
	mg.Deps(installEnvtest)
	assets, err := resolveEnvtestAssets()
	if err != nil {
		return err
	}
	// -count=1 disables the test cache: a merged -coverprofile across multiple packages
	// under-counts when some packages are served from cache, which would make the gate flaky.
	if err := sh.RunWithV(
		map[string]string{"KUBEBUILDER_ASSETS": assets},
		"go", "test", "./operator/...", "-count=1", "-coverpkg="+coverPkg, "-coverprofile="+coverProfile,
	); err != nil {
		return err
	}
	return CoverageGate()
}

// CoverageGate fails if total coverage of the logic packages is below the threshold.
func CoverageGate() error {
	out, err := sh.Output("go", "tool", "cover", "-func="+coverProfile)
	if err != nil {
		return err
	}
	var total float64
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "total:") {
			fields := strings.Fields(line)
			total, _ = strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
		}
	}
	fmt.Printf("total coverage: %.1f%% (min %.1f%%)\n", total, coverageMin)
	if total < coverageMin {
		return fmt.Errorf("coverage %.1f%% < %.1f%%", total, coverageMin)
	}
	return nil
}

// TestE2E runs the REAL-WORLD suite in test/e2e against a live cluster selected by the ambient
// KUBECONFIG. It requires the operator already installed on that cluster (see
// test/runbooks/verify-core-on-kind.md) and is build-tagged `e2e` so it never runs under Test.
// No coverage gate — these assert live behavior, not statement coverage.
func TestE2E() error {
	return sh.RunV("go", "test", "-tags", "e2e", "./test/e2e/...", "-v", "-count=1", "-timeout", "15m")
}

// TestE2EAWS runs the AWS greenfield real-world test: it drives the live/aws-full
// two-phase Terraform apply against a real AWS account (default profile
// c3.test.aws), asserts the satellite came up, and tears down. Build-tagged
// e2e_aws and gated on E2E_AWS=true, because it provisions a private EKS cluster +
// Network Firewall + NAT (~20-30 min, real cost). Build the preflight binary and
// create live/aws-full/terraform.tfvars first; see docs/operations/aws/deploy.md.
func TestE2EAWS() error {
	mg.Deps(PreflightBuild)
	return sh.RunV("go", "test", "-tags", "e2e_aws", "./test/e2e/...",
		"-run", "TestAWSFullGreenfield", "-v", "-count=1", "-timeout", "75m")
}

// TestE2EAWSNoEKS runs the no-EKS building-blocks test: it applies network + kms +
// iam + secrets (no cluster) against a real account (default profile c3.test.aws),
// asserts the module outputs + the preflight binary's AWS cloud stages against the
// live resources, then destroys. Build-tagged e2e_aws and gated on
// E2E_AWS_NOEKS=true. A few $ for NAT + Network Firewall, ~5-8 min. Build the
// preflight binary first (mage preflightBuild).
func TestE2EAWSNoEKS() error {
	mg.Deps(PreflightBuild)
	return sh.RunV("go", "test", "-tags", "e2e_aws", "./test/e2e/...",
		"-run", "TestAWSNoEKS", "-v", "-count=1", "-timeout", "40m")
}

// TestE2EGCP runs the GCP greenfield real-world test: it drives the live/gcp-full
// two-phase Terraform apply against a real GCP project (Application Default
// Credentials), asserts the satellite came up, and tears down. Build-tagged
// e2e_gcp and gated on E2E_GCP=true, because it provisions a private GKE cluster +
// Cloud NAT (~20-30 min, real cost). Build the preflight binary and create
// live/gcp-full/terraform.tfvars first; see docs/operations/gcp/deploy.md.
func TestE2EGCP() error {
	mg.Deps(PreflightBuild)
	return sh.RunV("go", "test", "-tags", "e2e_gcp", "./test/e2e/...",
		"-run", "TestGCPFullGreenfield", "-v", "-count=1", "-timeout", "75m")
}

// TestE2EGCPNoGKE runs the no-GKE building-blocks test: it applies project +
// network + kms + secrets (no cluster) against a real project (Application Default
// Credentials), asserts the module outputs + the preflight binary's GCP cloud
// stages against the live resources, then destroys. Build-tagged e2e_gcp and
// gated on E2E_GCP_NOGKE=true. A few $ for Cloud NAT, ~5-8 min. Build the
// preflight binary first (mage preflightBuild).
func TestE2EGCPNoGKE() error {
	mg.Deps(PreflightBuild)
	return sh.RunV("go", "test", "-tags", "e2e_gcp", "./test/e2e/...",
		"-run", "TestGCPNoGKE", "-v", "-count=1", "-timeout", "40m")
}

// LintCharts lints both Helm charts.
func LintCharts() error {
	if err := sh.RunV("helm", "lint", "charts/workload",
		"--set", "name=demo", "--set", "namespace=demo", "--set", "image=nginx:1.27"); err != nil {
		return err
	}
	return sh.RunV("helm", "lint", "charts/workload-operator")
}

// DockerBuild builds the operator image (repo-root context so the embedded chart resolves).
func DockerBuild() error {
	return sh.RunV("docker", "build", "-f", "operator/Dockerfile", "-t", operatorImage, ".")
}

// resolveEnvtestAssets returns the KUBEBUILDER_ASSETS path for the envtest control-plane
// binaries. It fetches from the upstream index first (works in CI, where nothing is cached),
// then falls back to installed-only (-i) so local runs still work offline once the binaries
// are cached.
func resolveEnvtestAssets() (string, error) {
	if assets, err := sh.Output(setupEnvtest, "use", envtestK8sVersion, "-p", "path"); err == nil {
		return assets, nil
	}
	assets, err := sh.Output(setupEnvtest, "use", envtestK8sVersion, "-i", "-p", "path")
	if err != nil {
		return "", fmt.Errorf("resolve envtest assets for k8s %s (online fetch failed and none installed; run `setup-envtest use %s`): %w",
			envtestK8sVersion, envtestK8sVersion, err)
	}
	return assets, nil
}

func installControllerGen() error {
	if _, err := os.Stat(controllerGen); err == nil {
		return nil
	}
	return sh.RunWithV(map[string]string{"GOBIN": localBin},
		"go", "install", "sigs.k8s.io/controller-tools/cmd/controller-gen@"+controllerToolsVersion)
}

func installEnvtest() error {
	if _, err := os.Stat(setupEnvtest); err == nil {
		return nil
	}
	return sh.RunWithV(map[string]string{"GOBIN": localBin},
		"go", "install", "sigs.k8s.io/controller-runtime/tools/setup-envtest@"+envtestVersion)
}

func mustAbs(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		panic(err)
	}
	return abs
}
