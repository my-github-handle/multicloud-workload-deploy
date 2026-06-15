package gcp

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/kms/apiv1/kmspb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

const testKey = "projects/demo/locations/us-central1/keyRings/r/cryptoKeys/k"

func timestamppbNow() *timestamppb.Timestamp { return timestamppb.Now() }

func TestCheckKMSKey(t *testing.T) {
	tests := []struct {
		name       string
		getKey     func(context.Context, *kmspb.GetCryptoKeyRequest) (*kmspb.CryptoKey, error)
		wantStatus preflight.Status
	}{
		{
			name: "enabled primary version with rotation -> green",
			getKey: func(_ context.Context, _ *kmspb.GetCryptoKeyRequest) (*kmspb.CryptoKey, error) {
				return &kmspb.CryptoKey{
					Primary: &kmspb.CryptoKeyVersion{
						State: kmspb.CryptoKeyVersion_ENABLED,
					},
					NextRotationTime: timestamppbNow(),
				}, nil
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name: "primary version disabled -> red",
			getKey: func(_ context.Context, _ *kmspb.GetCryptoKeyRequest) (*kmspb.CryptoKey, error) {
				return &kmspb.CryptoKey{
					Primary: &kmspb.CryptoKeyVersion{State: kmspb.CryptoKeyVersion_DISABLED},
				}, nil
			},
			wantStatus: preflight.StatusRed,
		},
		{
			name: "enabled but no rotation scheduled -> amber",
			getKey: func(_ context.Context, _ *kmspb.GetCryptoKeyRequest) (*kmspb.CryptoKey, error) {
				return &kmspb.CryptoKey{
					Primary:          &kmspb.CryptoKeyVersion{State: kmspb.CryptoKeyVersion_ENABLED},
					NextRotationTime: nil,
				}, nil
			},
			wantStatus: preflight.StatusAmber,
		},
		{
			name: "get error -> red",
			getKey: func(_ context.Context, _ *kmspb.GetCryptoKeyRequest) (*kmspb.CryptoKey, error) {
				return nil, errors.New("PermissionDenied")
			},
			wantStatus: preflight.StatusRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				ProjectID: "demo",
				Region:    "us-central1",
				KMSKeyID:  testKey,
				KMS:       stubKMS{getKey: tt.getKey},
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
