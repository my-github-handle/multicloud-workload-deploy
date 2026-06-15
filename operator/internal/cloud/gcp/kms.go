package gcp

import (
	"context"
	"fmt"

	"cloud.google.com/go/kms/apiv1/kmspb"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// CheckKMSKey implements Stage 1: the resolved CryptoKey exists, its primary
// version is ENABLED, and automatic rotation is scheduled. On GCP there is no
// key-level "enabled" flag — usability is the primary CryptoKeyVersion's state;
// rotation is NextRotationTime presence.
func (p *Provider) CheckKMSKey(ctx context.Context) []cloud.CheckResult {
	key, err := p.KMS.GetCryptoKey(ctx, &kmspb.GetCryptoKeyRequest{Name: p.KMSKeyID})
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "gcp.kms.get",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("cannot get CryptoKey %s: %v", p.KMSKeyID, err),
			Remediation: "grant cloudkms.cryptoKeys.get on the resolved key and confirm the resource id is correct",
		}}
	}
	if key.GetPrimary() == nil || key.GetPrimary().GetState() != kmspb.CryptoKeyVersion_ENABLED {
		return []cloud.CheckResult{{
			ID:          "gcp.kms.enabled",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("CryptoKey %s primary version is not ENABLED (state: %v)", p.KMSKeyID, primaryState(key)),
			Remediation: "enable the primary CryptoKeyVersion or supply an enabled key in BYO mode",
		}}
	}
	if key.GetNextRotationTime() == nil {
		return []cloud.CheckResult{{
			ID:          "gcp.kms.rotation",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("CryptoKey %s is enabled but no automatic rotation is scheduled", p.KMSKeyID),
			Remediation: "set a rotation_period on the CryptoKey (rotation policy)",
		}}
	}
	return []cloud.CheckResult{{
		ID:      "gcp.kms",
		Status:  preflight.StatusGreen,
		Message: fmt.Sprintf("CryptoKey %s enabled with rotation scheduled", p.KMSKeyID),
	}}
}

func primaryState(key *kmspb.CryptoKey) string {
	if key == nil || key.GetPrimary() == nil {
		return "unknown"
	}
	return key.GetPrimary().GetState().String()
}
