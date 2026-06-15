//go:build e2e_aws

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const awsNoEKSRelPath = "../../live/aws-noeks-test"

// tfIn runs a terraform command in the given dir with the AWS profile wired and
// ambient static creds cleared (so the profile's assume-role is used).
func tfIn(t *testing.T, dir string, timeout time.Duration, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := contextWithTimeout(timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = dir
	cmd.Env = awsProfileEnv()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// awsProfileEnv returns the process env with static AWS creds removed and the
// profile pinned, so terraform/SDK use the profile's assume-role.
func awsProfileEnv() []string {
	env := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "AWS_ACCESS_KEY_ID=") ||
			strings.HasPrefix(kv, "AWS_SECRET_ACCESS_KEY=") ||
			strings.HasPrefix(kv, "AWS_SESSION_TOKEN=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "AWS_PROFILE="+awsProfile())
}

func tfOut(t *testing.T, dir, name string) string {
	t.Helper()
	out, err := tfIn(t, dir, time.Minute, "output", "-raw", name)
	if err != nil {
		t.Fatalf("read output %q: %s", name, out)
	}
	return strings.TrimSpace(out)
}

// TestAWSNoEKS applies the cheap building blocks (network, kms, iam, secrets)
// against a real account WITHOUT an EKS cluster, asserts the module outputs, and
// runs the real preflight binary's AWS cloud stages against the live resources,
// then tears everything down. Gated on E2E_AWS_NOEKS=true (real AWS, a few $ for
// NAT + Network Firewall, ~5-8 min).
func TestAWSNoEKS(t *testing.T) {
	if os.Getenv("E2E_AWS_NOEKS") != "true" {
		t.Skip("set E2E_AWS_NOEKS=true to run the no-EKS building-blocks apply (real AWS, a few $, ~5-8 min)")
	}

	region := resolveRegion(t)

	t.Cleanup(func() {
		t.Log("tearing down aws-noeks-test (flow-log bucket may persist under Object Lock; see docs/operations/aws/deploy.md §5)")
		if out, err := tfIn(t, awsNoEKSRelPath, 20*time.Minute, "destroy", "-auto-approve", "-input=false"); err != nil {
			t.Logf("destroy returned an error (expected if the flow-log bucket is retention-locked):\n%s", out)
		}
	})

	if out, err := tfIn(t, awsNoEKSRelPath, 5*time.Minute, "init", "-input=false"); err != nil {
		t.Fatalf("terraform init failed:\n%s", out)
	}
	if out, err := tfIn(t, awsNoEKSRelPath, 15*time.Minute, "apply", "-input=false", "-auto-approve"); err != nil {
		t.Fatalf("apply failed:\n%s", out)
	}

	vpcID := tfOut(t, awsNoEKSRelPath, "vpc_id")
	kmsARN := tfOut(t, awsNoEKSRelPath, "kms_key_arn")
	egressRef := tfOut(t, awsNoEKSRelPath, "egress_path_ref")
	roleARN := tfOut(t, awsNoEKSRelPath, "role_arn")
	runtimePolicy := tfOut(t, awsNoEKSRelPath, "runtime_policy_json")

	// secret_arns is a JSON list; grab the first.
	var secretARNs []string
	if raw, err := tfIn(t, awsNoEKSRelPath, time.Minute, "output", "-json", "secret_arns"); err == nil {
		_ = json.Unmarshal([]byte(strings.TrimSpace(raw)), &secretARNs)
	}

	// --- Assert the module outputs are well-formed. ---
	if !strings.HasPrefix(vpcID, "vpc-") {
		t.Errorf("vpc_id = %q, want a vpc-… id", vpcID)
	}
	if !strings.Contains(egressRef, ":firewall/") {
		t.Errorf("egress_path_ref = %q, want a Network Firewall ARN", egressRef)
	}
	if !strings.Contains(kmsARN, ":key/") {
		t.Errorf("kms_key_arn = %q, want a KMS key ARN", kmsARN)
	}
	if !strings.HasPrefix(roleARN, "arn:aws:iam::") {
		t.Errorf("role_arn = %q, want an IAM role ARN", roleARN)
	}
	// Least-privilege runtime policy: scoped to the resolved key + secret prefix,
	// no service wildcards (the single Resource:* is ecr:GetAuthorizationToken).
	if !strings.Contains(runtimePolicy, kmsARN) {
		t.Errorf("runtime policy must scope KMS to the resolved key ARN")
	}
	if strings.Contains(runtimePolicy, "kms:*") || strings.Contains(runtimePolicy, "secretsmanager:*") {
		t.Errorf("runtime policy must not contain service wildcards")
	}
	if len(secretARNs) == 0 || !strings.Contains(secretARNs[0], ":secret:") {
		t.Fatalf("expected at least one Secrets Manager secret ARN, got %v", secretARNs)
	}

	// --- Run the real preflight binary's AWS provider (stages 0-3) against the
	//     live resources via --cloud=aws. No kubeconfig is passed, so the binary's
	//     Kubernetes stages (4-5) report red (no cluster) — that is expected here;
	//     we assert only on the CLOUD stage results (ids 0-3), which exercise the
	//     real SDK calls against the resources just provisioned. ---
	bin, err := filepath.Abs("../../operator/bin/preflight")
	if err != nil {
		t.Fatalf("resolve preflight binary: %v", err)
	}
	if _, statErr := os.Stat(bin); statErr != nil {
		t.Fatalf("preflight binary not found at %s — run `mage preflightBuild` first: %v", bin, statErr)
	}

	report := runPreflightCloud(t, bin, map[string]string{
		"AWS_REGION":                    region,
		"PREFLIGHT_AWS_KMS_KEY_ARN":     kmsARN,
		"PREFLIGHT_AWS_SECRET_ARNS":     strings.Join(secretARNs, ","),
		"PREFLIGHT_AWS_VPC_ID":          vpcID,
		"PREFLIGHT_AWS_EGRESS_PATH_REF": egressRef,
	})

	// Assert no CLOUD-stage (id 0-3) result is red against the live resources.
	for _, st := range report.Stages {
		if st.ID > 3 {
			continue // skip the Kubernetes stages (no cluster in this test)
		}
		for _, r := range st.Results {
			t.Logf("stage %d: %s = %s (%s)", st.ID, r.ID, r.Status, r.Message)
			if r.Status == "red" {
				t.Errorf("cloud stage %d check %s is RED against live resources: %s", st.ID, r.ID, r.Message)
			}
		}
	}

	t.Logf("no-EKS e2e OK: vpc=%s kms=%s firewall=%s role=%s", vpcID, kmsARN, egressRef, roleARN)
}

// preflightReport is the subset of the staged report this test inspects.
type preflightReport struct {
	Stages []struct {
		ID      int `json:"id"`
		Results []struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"results"`
	} `json:"stages"`
}

// runPreflightCloud runs the preflight binary with --cloud=aws (no kubeconfig) and
// the given extra env, then decodes the double-encoded report_json. The binary
// always exits 0 and prints the flat {verdict, report_json} map.
func runPreflightCloud(t *testing.T, bin string, extraEnv map[string]string) preflightReport {
	t.Helper()
	ctx, cancel := contextWithTimeout(3 * time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--cloud=aws", "--namespace", "workload-system")
	cmd.Env = awsProfileEnv()
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("preflight binary failed: %v\n%s", err, out)
	}
	var flat struct {
		Verdict    string `json:"verdict"`
		ReportJSON string `json:"report_json"`
	}
	if jerr := json.Unmarshal(out, &flat); jerr != nil {
		t.Fatalf("decode preflight flat output: %v\n%s", jerr, out)
	}
	var report preflightReport
	if jerr := json.Unmarshal([]byte(flat.ReportJSON), &report); jerr != nil {
		t.Fatalf("decode report_json: %v", jerr)
	}
	return report
}
