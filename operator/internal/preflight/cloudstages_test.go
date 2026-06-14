package preflight_test

import (
	"context"
	"testing"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/fake"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func TestCloudChecksMapToStages(t *testing.T) {
	p := &fake.Provider{
		IdentityResults: []cloud.CheckResult{{ID: "iam", Status: preflight.StatusGreen}},
		KMSResults:      []cloud.CheckResult{{ID: "kms", Status: preflight.StatusGreen}},
		SecretsResults:  []cloud.CheckResult{{ID: "secrets", Status: preflight.StatusAmber}},
		EgressResults:   []cloud.CheckResult{{ID: "egress", Status: preflight.StatusGreen}},
	}
	checks := preflight.CloudChecks(p)

	wantStage := map[string]preflight.StageID{
		"iam":     preflight.StageIdentity,
		"kms":     preflight.StageKMS,
		"secrets": preflight.StageSecrets,
		"egress":  preflight.StageNetwork,
	}
	if len(checks) != 4 {
		t.Fatalf("expected 4 cloud checks, got %d", len(checks))
	}
	ctx := context.Background()
	for _, c := range checks {
		res := c.Run(ctx)
		if len(res) != 1 {
			t.Fatalf("check %s returned %d results", c.Name(), len(res))
		}
		if want := wantStage[res[0].ID]; want != c.Stage() {
			t.Errorf("result %q is on stage %d, want %d", res[0].ID, c.Stage(), want)
		}
	}
}

func TestCloudChecksRunThroughRunner(t *testing.T) {
	p := &fake.Provider{
		KMSResults: []cloud.CheckResult{{ID: "kms.disabled", Status: preflight.StatusRed, Message: "key disabled"}},
	}
	r := preflight.NewRunner(preflight.CloudChecks(p))
	report := r.Run(context.Background())

	if report.Verdict != preflight.StatusRed {
		t.Errorf("verdict = %q, want red", report.Verdict)
	}
	// Stage 1 (KMS) is red; secrets/network/k8s/workload are skipped.
	if report.Stages[preflight.StageSecrets].Status != preflight.StatusSkipped {
		t.Errorf("secrets stage = %q, want skipped", report.Stages[preflight.StageSecrets].Status)
	}
	// Identity (stage 0) ran before the red and is green (fake default).
	if report.Stages[preflight.StageIdentity].Status != preflight.StatusGreen {
		t.Errorf("identity stage = %q, want green", report.Stages[preflight.StageIdentity].Status)
	}
}

// TestStage3EmitsStableEgressResultIDs locks the Stage-3 result-ID contract: the cloud egress
// check is NOT one opaque result — it emits a distinct, stable CheckResult per egress concern,
// which the per-cloud plans implement and the Terraform module keys on.
func TestStage3EmitsStableEgressResultIDs(t *testing.T) {
	egress := []cloud.CheckResult{
		{ID: "egress.metadata_block", Status: preflight.StatusGreen, Message: "IMDS blocked"},
		{ID: "egress.ghcr", Status: preflight.StatusGreen, Message: "ghcr.io reachable on allowed path"},
		{ID: "egress.cloud_api", Status: preflight.StatusGreen},
		{ID: "egress.observability", Status: preflight.StatusGreen},
		{ID: "egress.controlplane_fqdn", Status: preflight.StatusGreen, Message: "control-plane FQDN reachable"},
		{ID: "egress.firewall_inpath", Status: preflight.StatusGreen},
	}
	p := &fake.Provider{EgressResults: egress}
	checks := preflight.CloudChecks(p)

	var net preflight.Check
	for _, c := range checks {
		if c.Stage() == preflight.StageNetwork {
			net = c
		}
	}
	if net == nil {
		t.Fatal("no Stage 3 (network) check found")
	}
	got := map[string]bool{}
	for _, r := range net.Run(context.Background()) {
		got[r.ID] = true
	}
	for _, want := range []string{
		"egress.metadata_block", "egress.ghcr", "egress.cloud_api",
		"egress.observability", "egress.controlplane_fqdn", "egress.firewall_inpath",
	} {
		if !got[want] {
			t.Errorf("Stage 3 missing stable result ID %q (per-cloud contract)", want)
		}
	}
}

// TestStage3ControlPlaneFQDNBlockedBlocksDeploy proves egress.controlplane_fqdn is load-bearing:
// red (blocked) makes Stage 3 red, which (in agnostic/blocking mode) yields a red verdict — the
// connect-agent could not dial home, so the deploy must not proceed.
func TestStage3ControlPlaneFQDNBlockedBlocksDeploy(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cpStatus    preflight.Status
		wantStage3  preflight.Status
		wantVerdict preflight.Status
	}{
		{"reachable", preflight.StatusGreen, preflight.StatusGreen, preflight.StatusGreen},
		{"blocked", preflight.StatusRed, preflight.StatusRed, preflight.StatusRed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &fake.Provider{EgressResults: []cloud.CheckResult{
				{ID: "egress.metadata_block", Status: preflight.StatusGreen},
				{ID: "egress.controlplane_fqdn", Status: tc.cpStatus,
					Message: "connect-agent outbound tunnel reachability"},
			}}
			r := preflight.NewRunner(preflight.CloudChecks(p))
			report := r.Run(context.Background())
			if report.Stages[preflight.StageNetwork].Status != tc.wantStage3 {
				t.Errorf("stage 3 = %q, want %q", report.Stages[preflight.StageNetwork].Status, tc.wantStage3)
			}
			if report.Verdict != tc.wantVerdict {
				t.Errorf("verdict = %q, want %q", report.Verdict, tc.wantVerdict)
			}
		})
	}
}

// TestNoKubeconfigK8sChecksIsBlockingRed asserts the no-kubeconfig fallback emits a single
// blocking red Stage-4 check (the deploy target is unreachable), and that providerCheck.Name is
// wired through.
func TestNoKubeconfigK8sChecksIsBlockingRed(t *testing.T) {
	checks := preflight.NoKubeconfigK8sChecks()
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Stage() != preflight.StageKubernetes {
		t.Errorf("stage = %d, want kubernetes(4)", checks[0].Stage())
	}
	if checks[0].Name() == "" {
		t.Error("check name should not be empty")
	}
	res := checks[0].Run(context.Background())
	if len(res) != 1 || res[0].Status != preflight.StatusRed || res[0].ID != "k8s.unreachable" {
		t.Errorf("expected a single red k8s.unreachable result, got %+v", res)
	}
}

// TestCloudChecksNamesAreSet exercises providerCheck.Name for each cloud stage.
func TestCloudChecksNamesAreSet(t *testing.T) {
	for _, c := range preflight.CloudChecks(&fake.Provider{}) {
		if c.Name() == "" {
			t.Errorf("cloud check on stage %d has empty name", c.Stage())
		}
	}
}
