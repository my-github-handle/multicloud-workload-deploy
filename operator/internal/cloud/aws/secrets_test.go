package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func TestCheckSecretsBackend(t *testing.T) {
	const cmk = "arn:aws:kms:us-east-1:111122223333:key/abc"
	tests := []struct {
		name       string
		describe   func(context.Context, *secretsmanager.DescribeSecretInput) (*secretsmanager.DescribeSecretOutput, error)
		secretARNs []string
		wantStatus preflight.Status
	}{
		{
			name:       "no secrets configured -> amber",
			secretARNs: nil,
			wantStatus: preflight.StatusAmber,
		},
		{
			name:       "secret encrypted with resolved CMK -> green",
			secretARNs: []string{"arn:aws:secretsmanager:us-east-1:111122223333:secret:demo-db-AbCdEf"},
			describe: func(_ context.Context, _ *secretsmanager.DescribeSecretInput) (*secretsmanager.DescribeSecretOutput, error) {
				k := cmk
				return &secretsmanager.DescribeSecretOutput{KmsKeyId: &k}, nil
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name:       "secret encrypted with a DIFFERENT key -> red",
			secretARNs: []string{"arn:aws:secretsmanager:us-east-1:111122223333:secret:demo-db-AbCdEf"},
			describe: func(_ context.Context, _ *secretsmanager.DescribeSecretInput) (*secretsmanager.DescribeSecretOutput, error) {
				other := "arn:aws:kms:us-east-1:111122223333:key/OTHER"
				return &secretsmanager.DescribeSecretOutput{KmsKeyId: &other}, nil
			},
			wantStatus: preflight.StatusRed,
		},
		{
			name:       "describe error -> red",
			secretARNs: []string{"arn:aws:secretsmanager:us-east-1:111122223333:secret:demo-db-AbCdEf"},
			describe: func(_ context.Context, _ *secretsmanager.DescribeSecretInput) (*secretsmanager.DescribeSecretOutput, error) {
				return nil, errors.New("AccessDenied")
			},
			wantStatus: preflight.StatusRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				Region:     "us-east-1",
				KMSKeyARN:  cmk,
				SecretARNs: tt.secretARNs,
				Secrets:    stubSecrets{describe: tt.describe},
			}
			res := p.CheckSecretsBackend(context.Background())
			if len(res) == 0 {
				t.Fatal("expected a result")
			}
			if res[0].Status != tt.wantStatus {
				t.Errorf("status = %q, want %q (msg: %s)", res[0].Status, tt.wantStatus, res[0].Message)
			}
		})
	}
}
