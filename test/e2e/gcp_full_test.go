//go:build e2e_gcp

// Package e2e_gcp holds the real-world GCP greenfield test: it drives the
// `live/gcp-full` two-phase Terraform apply against a real GCP project, asserts
// the satellite came up, and tears everything down. It is guarded by its OWN
// build tag (`e2e_gcp`) AND an explicit opt-in env (E2E_GCP=true), because it
// provisions a private GKE cluster + Cloud NAT — ~20-30 min and real cost.
//
// Auth: it uses Application Default Credentials (gcloud auth
// application-default login). The project/region come from the tfvars.
//
// Run:
//
//	E2E_GCP=true go test -tags e2e_gcp ./test/e2e/ -run TestGCPFullGreenfield -v -timeout 75m
//
// or `mage testE2EGCP`.
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const gcpFullRelPath = "../../live/gcp-full"

// gctx is a tiny wrapper so the long-running terraform steps share one
// cancellation idiom.
func gctx(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// tfGCP runs a terraform command in the given dir, inheriting the ambient env
// (Application Default Credentials).
func tfGCP(t *testing.T, dir string, timeout time.Duration, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := gctx(timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func tfOutGCP(t *testing.T, dir, name string) string {
	t.Helper()
	out, err := tfGCP(t, dir, time.Minute, "output", "-raw", name)
	if err != nil {
		t.Fatalf("read output %q: %s", name, out)
	}
	return strings.TrimSpace(out)
}

func TestGCPFullGreenfield(t *testing.T) {
	if os.Getenv("E2E_GCP") != "true" {
		t.Skip("set E2E_GCP=true to run the GCP greenfield apply (real cost, ~20-30 min)")
	}

	binAbs, err := filepath.Abs("../../operator/bin/preflight")
	if err != nil {
		t.Fatalf("resolve preflight binary path: %v", err)
	}
	if _, err := os.Stat(binAbs); err != nil {
		t.Fatalf("preflight binary not found at %s — run `mage preflightBuild` first: %v", binAbs, err)
	}
	tfvars := filepath.Join(gcpFullRelPath, "terraform.tfvars")
	if _, err := os.Stat(tfvars); err != nil {
		t.Fatalf("live/gcp-full/terraform.tfvars not found — copy terraform.tfvars.example and edit it: %v", err)
	}

	// Always attempt teardown, even on failure. The flow-log bucket (locked
	// retention) and the CryptoKey (prevent_destroy) may survive; that is expected
	// (see docs/operations/gcp/deploy.md §7).
	t.Cleanup(func() {
		t.Log("tearing down gcp-full (best effort; flow-log bucket + CryptoKey may persist)")
		if out, err := tfGCP(t, gcpFullRelPath, 40*time.Minute, "destroy", "-auto-approve",
			"-target=module.workload", "-target=module.k8s_observability",
			"-target=module.k8s_security", "-target=module.k8s_platform",
			"-target=module.preflight", "-target=module.secrets", "-target=module.iam",
			"-target=module.cluster_resolver", "-target=module.cluster",
			"-target=module.network_resolver", "-target=module.network"); err != nil {
			t.Logf("destroy returned an error (expected for retention-locked/prevent_destroy resources):\n%s", out)
		}
	})

	if out, err := tfGCP(t, gcpFullRelPath, 5*time.Minute, "init", "-input=false"); err != nil {
		t.Fatalf("terraform init failed:\n%s", out)
	}

	// Single apply: provision the project + cloud infra + cluster AND deploy the
	// Layer-3 satellite in one pass. The in-cluster providers defer their
	// resources until the cluster's computed endpoint/CA are known, within this
	// same apply.
	if out, err := tfGCP(t, gcpFullRelPath, 60*time.Minute, "apply", "-input=false", "-auto-approve"); err != nil {
		t.Fatalf("apply failed:\n%s", out)
	}

	// --- Assert the headline outputs. ---
	verdict := tfOutGCP(t, gcpFullRelPath, "preflight_verdict")
	if verdict != "green" && verdict != "amber" {
		t.Errorf("preflight verdict = %q, want green or amber (full mode)", verdict)
	}

	tier := tfOutGCP(t, gcpFullRelPath, "install_tier")
	if tier != "A" {
		t.Errorf("install_tier = %q, want A on a freshly provisioned cluster", tier)
	}

	t.Logf("gcp-full greenfield apply succeeded: verdict=%s tier=%s project=%s",
		verdict, tier, tfOutGCP(t, gcpFullRelPath, "project_id"))
}
