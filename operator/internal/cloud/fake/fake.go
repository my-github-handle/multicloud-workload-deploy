// Package fake provides a configurable cloud.PreflightProvider for testing the preflight binary
// and report assembly without real cloud credentials.
package fake

import (
	"context"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
)

// Provider is a cloud.PreflightProvider whose method outputs are set by the caller. A nil slice
// yields a single green "stub" result so a zero-value Provider still produces a green report.
type Provider struct {
	IdentityResults []cloud.CheckResult
	KMSResults      []cloud.CheckResult
	SecretsResults  []cloud.CheckResult
	EgressResults   []cloud.CheckResult
}

func orDefault(results []cloud.CheckResult, id string) []cloud.CheckResult {
	if results != nil {
		return results
	}
	return []cloud.CheckResult{{
		ID:      id,
		Status:  "green",
		Message: "fake provider: not evaluated (stub green)",
	}}
}

func (p *Provider) CheckIdentityPermissions(ctx context.Context) []cloud.CheckResult {
	return orDefault(p.IdentityResults, "fake.identity")
}

func (p *Provider) CheckKMSKey(ctx context.Context) []cloud.CheckResult {
	return orDefault(p.KMSResults, "fake.kms")
}

func (p *Provider) CheckSecretsBackend(ctx context.Context) []cloud.CheckResult {
	return orDefault(p.SecretsResults, "fake.secrets")
}

func (p *Provider) CheckEgress(ctx context.Context) []cloud.CheckResult {
	return orDefault(p.EgressResults, "fake.egress")
}
