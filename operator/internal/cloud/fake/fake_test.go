package fake_test

import (
	"context"
	"testing"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/fake"
)

func TestFakeReturnsConfiguredResults(t *testing.T) {
	want := []cloud.CheckResult{
		{ID: "iam.missing", Status: "red", Message: "missing kms:Decrypt"},
	}
	p := &fake.Provider{
		IdentityResults: want,
		KMSResults:      []cloud.CheckResult{{ID: "kms.ok", Status: "green"}},
		SecretsResults:  []cloud.CheckResult{{ID: "secrets.ok", Status: "green"}},
		EgressResults:   []cloud.CheckResult{{ID: "egress.ok", Status: "green"}},
	}

	ctx := context.Background()
	got := p.CheckIdentityPermissions(ctx)
	if len(got) != 1 || got[0].ID != "iam.missing" || got[0].Status != "red" {
		t.Errorf("CheckIdentityPermissions = %+v, want %+v", got, want)
	}
	if p.CheckKMSKey(ctx)[0].ID != "kms.ok" {
		t.Error("CheckKMSKey did not return configured result")
	}
	if p.CheckSecretsBackend(ctx)[0].ID != "secrets.ok" {
		t.Error("CheckSecretsBackend did not return configured result")
	}
	if p.CheckEgress(ctx)[0].ID != "egress.ok" {
		t.Error("CheckEgress did not return configured result")
	}
}

// TestFakeSatisfiesProvider is a compile-time assertion that *fake.Provider implements
// cloud.PreflightProvider.
func TestFakeSatisfiesProvider(t *testing.T) {
	var _ cloud.PreflightProvider = (*fake.Provider)(nil)
}
