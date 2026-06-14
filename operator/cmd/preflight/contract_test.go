package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/fake"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// TestExternalProviderContract asserts the exact shape the hashicorp external data source
// consumes: a flat object whose every value is a string, with keys "verdict" and "report_json",
// where report_json decodes into a full Report.
func TestExternalProviderContract(t *testing.T) {
	out, err := run(context.Background(), options{mode: "agnostic", cloud: ""})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Decode into a generic map and assert every value is a JSON string.
	var generic map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &generic); err != nil {
		t.Fatalf("output is not a JSON object: %v", err)
	}
	if len(generic) != 2 {
		t.Fatalf("expected exactly 2 keys (verdict, report_json), got %d: %s", len(generic), out)
	}
	for key, raw := range generic {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Errorf("value for %q is not a JSON string (external provider rejects non-strings): %s", key, raw)
		}
	}

	var flat map[string]string
	if err := json.Unmarshal([]byte(out), &flat); err != nil {
		t.Fatalf("not a flat string map: %v", err)
	}
	switch flat["verdict"] {
	case "green", "amber", "red":
	default:
		t.Errorf("verdict %q not one of green|amber|red", flat["verdict"])
	}

	var report preflight.Report
	if err := json.Unmarshal([]byte(flat["report_json"]), &report); err != nil {
		t.Fatalf("report_json does not decode to a Report: %v", err)
	}
	if len(report.Stages) != 6 {
		t.Errorf("decoded report has %d stages, want 6", len(report.Stages))
	}
	if string(report.Verdict) != flat["verdict"] {
		t.Errorf("inner verdict %q != top-level verdict %q", report.Verdict, flat["verdict"])
	}
}

// TestContractRedVerdictPropagates ensures a red cloud result yields verdict "red" at the top
// level so the Terraform precondition can gate on it.
func TestContractRedVerdictPropagates(t *testing.T) {
	provider := &fake.Provider{
		IdentityResults: []cloud.CheckResult{{ID: "iam.missing", Status: preflight.StatusRed, Message: "missing kms:Decrypt"}},
	}
	report := preflight.NewRunner(preflight.CloudChecks(provider)).Run(context.Background())
	out, err := emit(report)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	var flat map[string]string
	_ = json.Unmarshal([]byte(out), &flat)
	if flat["verdict"] != "red" {
		t.Errorf("verdict = %q, want red", flat["verdict"])
	}
}
