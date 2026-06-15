package gcp

import (
	"context"
	"fmt"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// CheckSecretsBackend implements Stage 2: each resolved secret is reachable and
// CMEK-encrypted with the resolved CryptoKey.
func (p *Provider) CheckSecretsBackend(ctx context.Context) []cloud.CheckResult {
	if len(p.SecretIDs) == 0 {
		return []cloud.CheckResult{{
			ID:          "gcp.secrets",
			Status:      preflight.StatusAmber,
			Message:     "no secrets configured to validate",
			Remediation: "if the workload needs secrets, provide their ids; otherwise this is informational",
		}}
	}

	var results []cloud.CheckResult
	for _, id := range p.SecretIDs {
		secret, err := p.Secrets.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: id})
		if err != nil {
			results = append(results, cloud.CheckResult{
				ID:          "gcp.secrets.get",
				Status:      preflight.StatusRed,
				Message:     fmt.Sprintf("cannot get secret %s: %v", id, err),
				Remediation: "grant secretmanager.secrets.get on the secret and confirm the id",
			})
			continue
		}
		if !cmekMatches(secret, p.KMSKeyID) {
			results = append(results, cloud.CheckResult{
				ID:          "gcp.secrets.encryption",
				Status:      preflight.StatusRed,
				Message:     fmt.Sprintf("secret %s is not CMEK-encrypted with the resolved key (%s)", id, p.KMSKeyID),
				Remediation: "re-create the secret with a user-managed replica using the resolved CryptoKey for CMEK",
			})
			continue
		}
		results = append(results, cloud.CheckResult{
			ID:      "gcp.secrets",
			Status:  preflight.StatusGreen,
			Message: fmt.Sprintf("secret %s is CMEK-encrypted with the resolved key", id),
		})
	}
	return results
}

// cmekMatches reports whether every user-managed replica of the secret is
// encrypted with the resolved key. Automatic replication (no CMEK) returns false.
func cmekMatches(secret *secretmanagerpb.Secret, keyID string) bool {
	um := secret.GetReplication().GetUserManaged()
	if um == nil || len(um.GetReplicas()) == 0 {
		return false
	}
	for _, r := range um.GetReplicas() {
		if r.GetCustomerManagedEncryption().GetKmsKeyName() != keyID {
			return false
		}
	}
	return true
}
