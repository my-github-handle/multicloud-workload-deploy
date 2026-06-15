//go:build e2e_gcp

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

const gcpNoGKERelPath = "../../live/gcp-nogke-test"

// TestGCPNoGKE applies the cheap building blocks (project-resolve, network, kms,
// secrets, iam) against a real project WITHOUT a GKE cluster, asserts the module
// outputs, and runs the real preflight binary's GCP cloud stages against the live
// resources, then tears everything down. Gated on E2E_GCP_NOGKE=true (real GCP, a
// few $ for Cloud NAT, ~5-8 min).
func TestGCPNoGKE(t *testing.T) {
	if os.Getenv("E2E_GCP_NOGKE") != "true" {
		t.Skip("set E2E_GCP_NOGKE=true to run the no-GKE building-blocks apply (real GCP, a few $, ~5-8 min)")
	}

	t.Cleanup(func() {
		t.Log("tearing down gcp-nogke-test (the CryptoKey has prevent_destroy; the flow-log bucket has locked retention)")
		if out, err := tfGCP(t, gcpNoGKERelPath, 20*time.Minute, "destroy", "-auto-approve", "-input=false",
			"-target=module.iam", "-target=module.secrets",
			"-target=module.network_resolver", "-target=module.network"); err != nil {
			t.Logf("destroy returned an error (expected for retention-locked/prevent_destroy resources):\n%s", out)
		}
	})

	if out, err := tfGCP(t, gcpNoGKERelPath, 5*time.Minute, "init", "-input=false"); err != nil {
		t.Fatalf("terraform init failed:\n%s", out)
	}
	if out, err := tfGCP(t, gcpNoGKERelPath, 15*time.Minute, "apply", "-input=false", "-auto-approve"); err != nil {
		t.Fatalf("apply failed:\n%s", out)
	}

	projectID := tfOutGCP(t, gcpNoGKERelPath, "project_id")
	netSelfLink := tfOutGCP(t, gcpNoGKERelPath, "network_self_link")
	routerName := tfOutGCP(t, gcpNoGKERelPath, "router_name")
	egressRef := tfOutGCP(t, gcpNoGKERelPath, "egress_path_ref")
	kmsKeyID := tfOutGCP(t, gcpNoGKERelPath, "kms_key_id")
	gsaEmail := tfOutGCP(t, gcpNoGKERelPath, "gsa_email")
	runtimeRole := tfOutGCP(t, gcpNoGKERelPath, "runtime_role_json")

	var secretIDs []string
	if raw, err := tfGCP(t, gcpNoGKERelPath, time.Minute, "output", "-json", "secret_ids"); err == nil {
		_ = json.Unmarshal([]byte(strings.TrimSpace(raw)), &secretIDs)
	}

	// --- Assert the module outputs are well-formed. ---
	if !strings.Contains(netSelfLink, "/networks/") {
		t.Errorf("network_self_link = %q, want a network self-link", netSelfLink)
	}
	if !strings.Contains(kmsKeyID, "/cryptoKeys/") {
		t.Errorf("kms_key_id = %q, want a CryptoKey id", kmsKeyID)
	}
	if !strings.HasSuffix(gsaEmail, ".iam.gserviceaccount.com") {
		t.Errorf("gsa_email = %q, want a GSA email", gsaEmail)
	}
	if egressRef == "" {
		t.Errorf("egress_path_ref must be the firewall policy name")
	}
	// Least-privilege runtime role: no primitive roles, no wildcards.
	for _, bad := range []string{"roles/owner", "roles/editor", "roles/viewer", "*"} {
		if strings.Contains(runtimeRole, bad) {
			t.Errorf("runtime role must not contain %q", bad)
		}
	}
	if len(secretIDs) == 0 || !strings.Contains(secretIDs[0], "/secrets/") {
		t.Fatalf("expected at least one Secret Manager secret id, got %v", secretIDs)
	}

	// --- Run the real preflight binary's GCP provider (stages 0-3) against the
	//     live resources via --cloud=gcp. No kubeconfig is passed, so the binary's
	//     Kubernetes stages (4-5) report red (no cluster) — expected here; we
	//     assert only on the CLOUD stage results (ids 0-3). ---
	bin, err := filepath.Abs("../../operator/bin/preflight")
	if err != nil {
		t.Fatalf("resolve preflight binary: %v", err)
	}
	if _, statErr := os.Stat(bin); statErr != nil {
		t.Fatalf("preflight binary not found at %s — run `mage preflightBuild` first: %v", bin, statErr)
	}

	report := runPreflightCloudGCP(t, bin, map[string]string{
		"PREFLIGHT_GCP_PROJECT_ID":        projectID,
		"PREFLIGHT_GCP_REGION":            regionFromEnvOrDefault(),
		"PREFLIGHT_GCP_KMS_KEY_ID":        kmsKeyID,
		"PREFLIGHT_GCP_SECRET_IDS":        strings.Join(secretIDs, ","),
		"PREFLIGHT_GCP_NETWORK_SELF_LINK": netSelfLink,
		"PREFLIGHT_GCP_ROUTER_NAME":       routerName,
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

	t.Logf("no-GKE e2e OK: project=%s network=%s kms=%s gsa=%s", projectID, netSelfLink, kmsKeyID, gsaEmail)
}

func regionFromEnvOrDefault() string {
	if r := os.Getenv("CLOUDSDK_COMPUTE_REGION"); r != "" {
		return r
	}
	return "us-central1"
}

// runPreflightCloudGCP runs the preflight binary with --cloud=gcp (no kubeconfig)
// and the given extra env, then decodes the double-encoded report_json. The binary
// always exits 0 and prints the flat {verdict, report_json} map.
func runPreflightCloudGCP(t *testing.T, bin string, extraEnv map[string]string) preflightReport {
	t.Helper()
	ctx, cancel := gctx(3 * time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--cloud=gcp", "--namespace", "workload-system")
	cmd.Env = os.Environ()
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
