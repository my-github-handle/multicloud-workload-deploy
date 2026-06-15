//go:build e2e_aws

// Package e2e_aws holds the real-world AWS greenfield test: it drives the
// `live/aws-full` two-phase Terraform apply against a real AWS account, asserts
// the satellite came up, and tears everything down. It is guarded by its OWN
// build tag (`e2e_aws`, separate from the cluster-only `e2e` suite) AND an
// explicit opt-in env (E2E_AWS=true), because it provisions a private EKS
// cluster + Network Firewall + NAT — ~20-30 min and real cost.
//
// Auth: it uses the AWS profile in AWS_PROFILE (default "c3.test.aws", which
// assumes the test role in the C3 test account via ~/.aws/config +
// ~/.aws/credentials). The shell's ambient AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY
// would otherwise shadow the profile, so the test clears them and relies on the
// profile alone.
//
// Run:
//
//	E2E_AWS=true go test -tags e2e_aws ./test/e2e/ -run TestAWSFullGreenfield -v -timeout 60m
//
// or `mage testE2EAWS`.
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

// contextWithTimeout is a tiny wrapper so the long-running terraform steps share
// one cancellation idiom.
func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

const awsFullRelPath = "../../live/aws-full"

// awsProfile returns the profile the test authenticates with (default the C3
// test-account profile that assumes the test role).
func awsProfile() string {
	if p := os.Getenv("AWS_PROFILE"); p != "" {
		return p
	}
	return "c3.test.aws"
}

// tf runs a terraform command in live/aws-full with the AWS profile wired and the
// ambient static creds cleared so the profile's assume-role is used.
func tf(t *testing.T, timeout time.Duration, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := contextWithTimeout(timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = awsFullRelPath
	// Start from the current env, drop static creds, pin the profile.
	env := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "AWS_ACCESS_KEY_ID=") ||
			strings.HasPrefix(kv, "AWS_SECRET_ACCESS_KEY=") ||
			strings.HasPrefix(kv, "AWS_SESSION_TOKEN=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "AWS_PROFILE="+awsProfile())
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestAWSFullGreenfield(t *testing.T) {
	if os.Getenv("E2E_AWS") != "true" {
		t.Skip("set E2E_AWS=true to run the AWS greenfield apply (real cost, ~20-30 min)")
	}

	// The preflight binary must be built and the tfvars present.
	binAbs, err := filepath.Abs("../../operator/bin/preflight")
	if err != nil {
		t.Fatalf("resolve preflight binary path: %v", err)
	}
	if _, err := os.Stat(binAbs); err != nil {
		t.Fatalf("preflight binary not found at %s — run `mage preflightBuild` first: %v", binAbs, err)
	}
	tfvars := filepath.Join(awsFullRelPath, "terraform.tfvars")
	if _, err := os.Stat(tfvars); err != nil {
		t.Fatalf("live/aws-full/terraform.tfvars not found — copy terraform.tfvars.example and edit it: %v", err)
	}

	kubeconfig := filepath.Join(os.TempDir(), "aws-full-e2e.kubeconfig")

	// Always attempt teardown, even on failure, so a broken run does not strand
	// the expensive infra. (The flow-log bucket may survive due to Object Lock;
	// see docs/operations/aws/deploy.md §5.)
	t.Cleanup(func() {
		t.Log("tearing down aws-full (best effort; flow-log bucket may persist under Object Lock)")
		if out, err := tf(t, 40*time.Minute, "destroy", "-auto-approve"); err != nil {
			t.Logf("destroy returned an error (expected if the flow-log bucket is retention-locked):\n%s", out)
		}
	})

	if out, err := tf(t, 5*time.Minute, "init", "-input=false"); err != nil {
		t.Fatalf("terraform init failed:\n%s", out)
	}

	// --- Phase 1: cloud infra incl. the cluster (no SecretProviderClass). ---
	phase1 := []string{
		"apply", "-input=false", "-auto-approve",
		"-target=module.network", "-target=module.network_resolver",
		"-target=module.kms", "-target=module.iam", "-target=module.secrets",
		"-target=module.cluster", "-target=module.cluster_resolver",
	}
	if out, err := tf(t, 40*time.Minute, phase1...); err != nil {
		t.Fatalf("phase 1 apply failed:\n%s", out)
	}

	// Write the kubeconfig the preflight binary + Layer-3 providers use.
	clusterName, err := tf(t, time.Minute, "output", "-raw", "workload_name")
	if err != nil {
		t.Fatalf("read workload_name output: %s", clusterName)
	}
	if out := updateKubeconfig(t, strings.TrimSpace(clusterName), kubeconfig); out != "" {
		t.Logf("update-kubeconfig: %s", out)
	}

	// --- Phase 2: Layer 3 + the SecretProviderClass (+ optional Cilium chaining). ---
	phase2 := []string{
		"apply", "-input=false", "-auto-approve",
		"-var", "create_secret_provider_class=true",
		"-var", "kubeconfig_path=" + kubeconfig,
	}
	if out, err := tf(t, 30*time.Minute, phase2...); err != nil {
		t.Fatalf("phase 2 apply failed:\n%s", out)
	}

	// --- Assert the headline outputs. ---
	verdict, err := tf(t, time.Minute, "output", "-raw", "preflight_verdict")
	if err != nil {
		t.Fatalf("read preflight_verdict: %s", verdict)
	}
	verdict = strings.TrimSpace(verdict)
	if verdict != "green" && verdict != "amber" {
		t.Errorf("preflight verdict = %q, want green or amber (full mode)", verdict)
	}

	tier, err := tf(t, time.Minute, "output", "-raw", "install_tier")
	if err != nil {
		t.Fatalf("read install_tier: %s", tier)
	}
	if strings.TrimSpace(tier) != "A" {
		t.Errorf("install_tier = %q, want A on a freshly provisioned cluster", strings.TrimSpace(tier))
	}

	t.Logf("aws-full greenfield apply succeeded: verdict=%s tier=%s cluster=%s",
		verdict, strings.TrimSpace(tier), strings.TrimSpace(clusterName))
}

// resolveRegion returns the region the AWS stack is deployed to: AWS_REGION if
// set, else the region the profile resolves (`aws configure get region`, which is
// what the Terraform aws provider also uses). It fails the test rather than
// silently defaulting to a region that may not match the deployed stack.
func resolveRegion(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("AWS_REGION"); r != "" {
		return r
	}
	ctx, cancel := contextWithTimeout(time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "aws", "configure", "get", "region")
	cmd.Env = awsProfileEnv()
	out, err := cmd.CombinedOutput()
	if r := strings.TrimSpace(string(out)); err == nil && r != "" {
		return r
	}
	t.Fatalf("could not resolve AWS region — set AWS_REGION or a region in the %q profile", awsProfile())
	return ""
}

// updateKubeconfig writes a kubeconfig for the provisioned EKS cluster using the
// same profile and region the apply ran under.
func updateKubeconfig(t *testing.T, clusterName, kubeconfig string) string {
	t.Helper()
	region := resolveRegion(t)
	ctx, cancel := contextWithTimeout(3 * time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "aws", "eks", "update-kubeconfig",
		"--name", clusterName, "--region", region, "--kubeconfig", kubeconfig)
	cmd.Env = awsProfileEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("aws eks update-kubeconfig failed: %s", out)
	}
	return strings.TrimSpace(string(out))
}
