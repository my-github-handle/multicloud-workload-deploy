package azure

import (
	"context"
	"fmt"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// CheckKMSKey implements Stage 1: the resolved Key Vault key exists, is enabled,
// and has a rotation policy (an expiry indicates rotation is configured).
func (p *Provider) CheckKMSKey(ctx context.Context) []cloud.CheckResult {
	out, err := p.Keys.GetKey(ctx, p.KeyName, "", nil)
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "azure.kms.get",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("cannot get Key Vault key %s in %s: %v", p.KeyName, p.KeyVaultURI, err),
			Remediation: "grant the deploy identity Key Vault Crypto/Reader on the resolved vault and confirm the key name",
		}}
	}
	attrs := out.Attributes
	if attrs == nil || attrs.Enabled == nil || !*attrs.Enabled {
		return []cloud.CheckResult{{
			ID:          "azure.kms.enabled",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("Key Vault key %s is not enabled", p.KeyName),
			Remediation: "enable the key or supply an enabled key in BYO mode",
		}}
	}
	// An expiry set on the key indicates a rotation policy is in effect (the kms
	// module's rotation_policy sets expire_after). No expiry -> rotation unknown.
	if attrs.Expires == nil {
		return []cloud.CheckResult{{
			ID:          "azure.kms.rotation",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("Key Vault key %s is enabled but no rotation/expiry policy is set", p.KeyName),
			Remediation: "configure an automatic rotation policy",
		}}
	}

	return []cloud.CheckResult{{
		ID:      "azure.kms",
		Status:  preflight.StatusGreen,
		Message: fmt.Sprintf("Key Vault key %s enabled with a rotation/expiry policy", p.KeyName),
	}}
}
