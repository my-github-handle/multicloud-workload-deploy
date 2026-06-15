package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	awsprovider "github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/aws"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/fake"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func TestSelectProviderDefaultsToFakeWhenCloudUnset(t *testing.T) {
	p, err := selectProvider("")
	if err != nil {
		t.Fatalf("selectProvider(\"\"): %v", err)
	}
	if _, ok := p.(*fake.Provider); !ok {
		t.Errorf("expected *fake.Provider when --cloud unset, got %T", p)
	}
}

func TestSelectProviderRealCloudNotYetImplemented(t *testing.T) {
	for _, c := range []string{"gcp", "azure"} {
		if _, err := selectProvider(c); err == nil {
			t.Errorf("selectProvider(%q) = nil error, want not-implemented error", c)
		}
	}
}

// TestSelectProviderAWSReturnsRealProvider asserts the real AWS provider is wired
// (it previously returned a not-implemented error).
func TestSelectProviderAWSReturnsRealProvider(t *testing.T) {
	p, err := selectProvider("aws")
	if err != nil {
		t.Fatalf("selectProvider(\"aws\") returned error: %v", err)
	}
	if _, ok := p.(*awsprovider.Provider); !ok {
		t.Fatalf("expected *aws.Provider, got %T", p)
	}
}

func TestSplitNonEmpty(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ,, c ", []string{"a", "b", "c"}}, // trims spaces, drops empties
		{",,", nil},
	}
	for _, c := range cases {
		got := splitNonEmpty(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitNonEmpty(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitNonEmpty(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestEmitFlatStringMap(t *testing.T) {
	report := preflight.Report{
		Verdict: preflight.StatusAmber,
		Stages: []preflight.Stage{
			{ID: 0, Name: "identity", Status: preflight.StatusAmber,
				Results: []preflight.CheckResult{{ID: "iam", Status: preflight.StatusAmber, Message: "excess"}}},
		},
	}
	out, err := emit(report)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	// The external provider requires a flat map of string→string.
	var flat map[string]string
	if err := json.Unmarshal([]byte(out), &flat); err != nil {
		t.Fatalf("output is not a flat string map: %v\noutput: %s", err, out)
	}
	if flat["verdict"] != "amber" {
		t.Errorf("verdict = %q, want amber", flat["verdict"])
	}
	if _, ok := flat["report_json"]; !ok {
		t.Fatal("missing report_json key")
	}
	// report_json must itself be a JSON string that decodes back into a Report.
	var got preflight.Report
	if err := json.Unmarshal([]byte(flat["report_json"]), &got); err != nil {
		t.Fatalf("report_json is not valid JSON: %v", err)
	}
	if got.Verdict != preflight.StatusAmber || len(got.Stages) != 1 {
		t.Errorf("decoded report mismatch: %+v", got)
	}
}

func TestRunAgnosticModeWithFakeProducesGreen(t *testing.T) {
	// Cloud-only path: with no kubeconfig AND the test-only skip set, only the fake cloud stages
	// run (k8s stages omitted), so the fake defaults yield green.
	out, err := run(context.Background(), options{mode: "agnostic", cloud: "", kubeconfig: "", skipK8sWhenNoKubeconfig: true})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(out), &flat); err != nil {
		t.Fatalf("output not flat string map: %v", err)
	}
	if flat["verdict"] != "green" {
		t.Errorf("verdict = %q, want green (fake defaults, k8s skipped in test)", flat["verdict"])
	}
}

// TestRunAgnosticNoKubeconfigIsRed: a real agnostic invocation (skipK8sWhenNoKubeconfig=false)
// with no kubeconfig must NOT emit a false green — the kubernetes deploy target is unreachable,
// so verdict=red.
func TestRunAgnosticNoKubeconfigIsRed(t *testing.T) {
	out, err := run(context.Background(), options{mode: "agnostic", cloud: "", kubeconfig: ""})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(out), &flat); err != nil {
		t.Fatalf("output not flat string map: %v", err)
	}
	if flat["verdict"] != "red" {
		t.Errorf("verdict = %q, want red (no kubeconfig → unreachable deploy target)", flat["verdict"])
	}
	var report preflight.Report
	_ = json.Unmarshal([]byte(flat["report_json"]), &report)
	if report.Stages[preflight.StageKubernetes].Status != preflight.StatusRed {
		t.Errorf("stage 4 = %q, want red", report.Stages[preflight.StageKubernetes].Status)
	}
}

func TestRunUnknownCloudErrors(t *testing.T) {
	if _, err := run(context.Background(), options{mode: "agnostic", cloud: "saturn"}); err == nil {
		t.Error("run with unknown --cloud should error")
	}
}

func TestRunBadKubeconfigErrors(t *testing.T) {
	// A non-existent kubeconfig path makes buildKubeClients fail; run returns the error, which
	// main surfaces via emitError.
	_, err := run(context.Background(), options{mode: "agnostic", kubeconfig: "/nonexistent/kubeconfig"})
	if err == nil {
		t.Error("run with a bad kubeconfig path should error")
	}
}

func TestBuildKubeClientsBadPath(t *testing.T) {
	if _, _, _, err := buildKubeClients("/nonexistent/kubeconfig"); err == nil {
		t.Error("buildKubeClients with a bad path should error")
	}
}

func TestRunFullModeGreenPath(t *testing.T) {
	// Smoke test of the full-mode branch of run(): with the fake's green defaults and k8s skipped,
	// the verdict is green. (Red-masking itself is asserted in TestFullModeMasksProvisionedRed,
	// which can inject a red cloud stage.)
	out, err := run(context.Background(), options{mode: "full", cloud: "", skipK8sWhenNoKubeconfig: true})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var flat map[string]string
	_ = json.Unmarshal([]byte(out), &flat)
	if flat["verdict"] != "green" {
		t.Errorf("verdict = %q, want green", flat["verdict"])
	}
}

// TestFullModeMasksProvisionedRed injects a genuinely red cloud stage and asserts full mode masks
// its gating effect: the verdict is amber (not red), the stage keeps its true red severity, and
// the run does not short-circuit. This is the regression test the green-path smoke test cannot be.
func TestFullModeMasksProvisionedRed(t *testing.T) {
	provider := &fake.Provider{
		KMSResults: []cloud.CheckResult{{ID: "kms.key", Status: preflight.StatusRed, Message: "BYO key disabled"}},
	}
	report := preflight.NewRunner(preflight.CloudChecks(provider)).
		RunWithProvisionedStages(context.Background(),
			preflight.StageIdentity, preflight.StageKMS, preflight.StageSecrets, preflight.StageNetwork)

	if report.Verdict != preflight.StatusAmber {
		t.Errorf("verdict = %q, want amber (provisioned red masked)", report.Verdict)
	}
	if report.Stages[preflight.StageKMS].Status != preflight.StatusRed {
		t.Errorf("kms stage = %q, want red (true severity preserved)", report.Stages[preflight.StageKMS].Status)
	}
	if report.Stages[preflight.StageKMS].Blocking {
		t.Error("kms stage should be non-blocking in full mode")
	}
	// Did not short-circuit: a later stage (secrets) still ran (green from the fake default).
	if report.Stages[preflight.StageSecrets].Status != preflight.StatusGreen {
		t.Errorf("secrets stage = %q, want green (full mode does not short-circuit on provisioned red)", report.Stages[preflight.StageSecrets].Status)
	}
}

func TestEmitErrorIsGateableRedFlatMap(t *testing.T) {
	// On an internal run() failure the binary must still emit a well-formed flat string map with
	// verdict=red so the Terraform external data source can read and gate on it.
	out := emitError(errors.New("boom: bad kubeconfig"))

	var flat map[string]string
	if err := json.Unmarshal([]byte(out), &flat); err != nil {
		t.Fatalf("emitError output is not a flat string map: %v\noutput: %s", err, out)
	}
	if flat["verdict"] != "red" {
		t.Errorf("verdict = %q, want red", flat["verdict"])
	}
	var got preflight.Report
	if err := json.Unmarshal([]byte(flat["report_json"]), &got); err != nil {
		t.Fatalf("report_json is not valid JSON: %v", err)
	}
	if got.Verdict != preflight.StatusRed {
		t.Errorf("decoded report verdict = %q, want red", got.Verdict)
	}
	// The original error must be carried into the report so the operator sees why.
	if len(got.Stages) == 0 || len(got.Stages[0].Results) == 0 ||
		!strings.Contains(got.Stages[0].Results[0].Message, "boom: bad kubeconfig") {
		t.Errorf("expected the run error to be carried in the report message, got %+v", got)
	}
}
