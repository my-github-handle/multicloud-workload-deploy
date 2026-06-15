package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// CheckSecretsBackend implements Stage 2: each resolved secret is reachable and
// envelope-encrypted with the resolved CMK.
func (p *Provider) CheckSecretsBackend(ctx context.Context) []cloud.CheckResult {
	if len(p.SecretARNs) == 0 {
		return []cloud.CheckResult{{
			ID:          "aws.secrets",
			Status:      preflight.StatusAmber,
			Message:     "no secrets configured to validate",
			Remediation: "if the workload needs secrets, provide their ARNs; otherwise this is informational",
		}}
	}

	var results []cloud.CheckResult
	for _, arn := range p.SecretARNs {
		out, err := p.Secrets.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{SecretId: ptr(arn)})
		if err != nil {
			results = append(results, cloud.CheckResult{
				ID:          "aws.secrets.describe",
				Status:      preflight.StatusRed,
				Message:     fmt.Sprintf("cannot describe secret %s: %v", arn, err),
				Remediation: "grant secretsmanager:DescribeSecret on the secret and confirm the ARN",
			})
			continue
		}
		if out.KmsKeyId == nil || *out.KmsKeyId != p.KMSKeyARN {
			results = append(results, cloud.CheckResult{
				ID:          "aws.secrets.encryption",
				Status:      preflight.StatusRed,
				Message:     fmt.Sprintf("secret %s is not encrypted with the resolved CMK (%s)", arn, p.KMSKeyARN),
				Remediation: "re-create or update the secret to use the resolved CMK for envelope encryption",
			})
			continue
		}
		results = append(results, cloud.CheckResult{
			ID:      "aws.secrets",
			Status:  preflight.StatusGreen,
			Message: fmt.Sprintf("secret %s is CMK-encrypted with the resolved key", arn),
		})
	}
	return results
}
