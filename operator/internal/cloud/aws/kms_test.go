package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func TestCheckKMSKey(t *testing.T) {
	tests := []struct {
		name       string
		describe   func(context.Context, *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error)
		rotation   func(context.Context, *kms.GetKeyRotationStatusInput) (*kms.GetKeyRotationStatusOutput, error)
		wantStatus preflight.Status
	}{
		{
			name: "enabled key with rotation -> green",
			describe: func(_ context.Context, _ *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error) {
				return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{
					Enabled: true, KeyState: kmstypes.KeyStateEnabled,
				}}, nil
			},
			rotation: func(_ context.Context, _ *kms.GetKeyRotationStatusInput) (*kms.GetKeyRotationStatusOutput, error) {
				return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: true}, nil
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name: "enabled key without rotation -> amber",
			describe: func(_ context.Context, _ *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error) {
				return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{
					Enabled: true, KeyState: kmstypes.KeyStateEnabled,
				}}, nil
			},
			rotation: func(_ context.Context, _ *kms.GetKeyRotationStatusInput) (*kms.GetKeyRotationStatusOutput, error) {
				return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: false}, nil
			},
			wantStatus: preflight.StatusAmber,
		},
		{
			name: "disabled key -> red",
			describe: func(_ context.Context, _ *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error) {
				return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{
					Enabled: false, KeyState: kmstypes.KeyStateDisabled,
				}}, nil
			},
			wantStatus: preflight.StatusRed,
		},
		{
			name: "describe error -> red",
			describe: func(_ context.Context, _ *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error) {
				return nil, errors.New("AccessDenied")
			},
			wantStatus: preflight.StatusRed,
		},
	}

	// No CMK configured: stage is not applicable, not a failure.
	t.Run("no key configured -> amber (skipped)", func(t *testing.T) {
		p := &Provider{Region: "us-east-1", KMSKeyARN: "", KMS: stubKMS{}}
		res := p.CheckKMSKey(context.Background())
		if len(res) == 0 || res[0].Status != preflight.StatusAmber {
			t.Fatalf("empty KMSKeyARN must yield a single amber result, got %+v", res)
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				Region:    "us-east-1",
				KMSKeyARN: "arn:aws:kms:us-east-1:111122223333:key/abc",
				KMS:       stubKMS{describe: tt.describe, rotation: tt.rotation},
			}
			res := p.CheckKMSKey(context.Background())
			if len(res) == 0 {
				t.Fatal("expected at least one result")
			}
			if res[0].Status != tt.wantStatus {
				t.Errorf("status = %q, want %q (msg: %s)", res[0].Status, tt.wantStatus, res[0].Message)
			}
		})
	}
}
