package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/kms"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// CheckKMSKey implements Stage 1: the resolved CMK exists, is enabled, and has
// automatic rotation on. When no key is configured (no envelope-encryption concern
// in play), the stage is not applicable rather than a failure.
func (p *Provider) CheckKMSKey(ctx context.Context) []cloud.CheckResult {
	if p.KMSKeyARN == "" {
		return []cloud.CheckResult{{
			ID:          "aws.kms",
			Status:      preflight.StatusAmber,
			Message:     "no CMK configured; key-encryption posture not applicable (nothing is envelope-encrypted by us)",
			Remediation: "if the workload's secrets/volumes must be CMK-encrypted, supply the resolved key ARN; otherwise this stage is informational",
		}}
	}

	out, err := p.KMS.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: ptr(p.KMSKeyARN)})
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "aws.kms.describe",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("cannot describe KMS key %s: %v", p.KMSKeyARN, err),
			Remediation: "grant kms:DescribeKey on the resolved key and confirm the ARN is correct",
		}}
	}
	if out.KeyMetadata == nil || !out.KeyMetadata.Enabled {
		return []cloud.CheckResult{{
			ID:          "aws.kms.enabled",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("KMS key %s is not enabled (state: %v)", p.KMSKeyARN, keyState(out)),
			Remediation: "enable the CMK or supply an enabled key in BYO mode",
		}}
	}

	rot, err := p.KMS.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{KeyId: ptr(p.KMSKeyARN)})
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "aws.kms.rotation",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("KMS key enabled but rotation status unknown: %v", err),
			Remediation: "grant kms:GetKeyRotationStatus, or confirm rotation manually",
		}}
	}
	if !rot.KeyRotationEnabled {
		return []cloud.CheckResult{{
			ID:          "aws.kms.rotation",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("KMS key %s is enabled but automatic rotation is OFF", p.KMSKeyARN),
			Remediation: "enable annual key rotation",
		}}
	}

	return []cloud.CheckResult{{
		ID:      "aws.kms",
		Status:  preflight.StatusGreen,
		Message: fmt.Sprintf("KMS key %s enabled with rotation on", p.KMSKeyARN),
	}}
}

func keyState(out *kms.DescribeKeyOutput) string {
	if out == nil || out.KeyMetadata == nil {
		return "unknown"
	}
	return string(out.KeyMetadata.KeyState)
}
