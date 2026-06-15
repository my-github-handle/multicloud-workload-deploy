package azure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func boolp(b bool) *bool { return &b }

func TestCheckKMSKey(t *testing.T) {
	future := time.Now().Add(90 * 24 * time.Hour)
	tests := []struct {
		name       string
		get        func(context.Context, string, string) (azkeys.GetKeyResponse, error)
		wantStatus preflight.Status
	}{
		{
			name: "enabled key with rotation policy (expires in future) -> green",
			get: func(_ context.Context, _, _ string) (azkeys.GetKeyResponse, error) {
				return azkeys.GetKeyResponse{KeyBundle: azkeys.KeyBundle{
					Attributes: &azkeys.KeyAttributes{Enabled: boolp(true), Expires: &future},
				}}, nil
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name: "disabled key -> red",
			get: func(_ context.Context, _, _ string) (azkeys.GetKeyResponse, error) {
				return azkeys.GetKeyResponse{KeyBundle: azkeys.KeyBundle{
					Attributes: &azkeys.KeyAttributes{Enabled: boolp(false)},
				}}, nil
			},
			wantStatus: preflight.StatusRed,
		},
		{
			name: "enabled key without expiry/rotation -> amber",
			get: func(_ context.Context, _, _ string) (azkeys.GetKeyResponse, error) {
				return azkeys.GetKeyResponse{KeyBundle: azkeys.KeyBundle{
					Attributes: &azkeys.KeyAttributes{Enabled: boolp(true)},
				}}, nil
			},
			wantStatus: preflight.StatusAmber,
		},
		{
			name: "get error -> red",
			get: func(_ context.Context, _, _ string) (azkeys.GetKeyResponse, error) {
				return azkeys.GetKeyResponse{}, errors.New("Forbidden")
			},
			wantStatus: preflight.StatusRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				KeyVaultURI: "https://demo.vault.azure.net/",
				KeyName:     "workload-cmk",
				Keys:        stubKeys{get: tt.get},
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
