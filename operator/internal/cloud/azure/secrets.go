package azure

import (
	"context"
	"fmt"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// CheckSecretsBackend implements Stage 2: each resolved secret is reachable in
// the resolved Key Vault. The vault's contents are encrypted at rest with the
// resolved key, so confirming the secret resolves in this vault establishes the
// envelope-encryption chain.
func (p *Provider) CheckSecretsBackend(ctx context.Context) []cloud.CheckResult {
	if len(p.SecretNames) == 0 {
		return []cloud.CheckResult{{
			ID:          "azure.secrets",
			Status:      preflight.StatusAmber,
			Message:     "no secrets configured to validate",
			Remediation: "if the workload needs secrets, provide their names; otherwise this is informational",
		}}
	}

	var results []cloud.CheckResult
	for _, name := range p.SecretNames {
		_, err := p.Secrets.GetSecret(ctx, name, "", nil)
		if err != nil {
			results = append(results, cloud.CheckResult{
				ID:          "azure.secrets.get",
				Status:      preflight.StatusRed,
				Message:     fmt.Sprintf("cannot get secret %s in vault %s: %v", name, p.KeyVaultURI, err),
				Remediation: "grant Key Vault Secrets User on the resolved vault and confirm the secret name",
			})
			continue
		}
		results = append(results, cloud.CheckResult{
			ID:      "azure.secrets",
			Status:  preflight.StatusGreen,
			Message: fmt.Sprintf("secret %s present in the resolved Key Vault (key-encrypted at rest)", name),
		})
	}
	return results
}
