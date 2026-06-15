package azure

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func TestCheckSecretsBackend(t *testing.T) {
	tests := []struct {
		name        string
		secretNames []string
		get         func(context.Context, string, string) (azsecrets.GetSecretResponse, error)
		wantStatus  preflight.Status
	}{
		{
			name:        "no secrets configured -> amber",
			secretNames: nil,
			wantStatus:  preflight.StatusAmber,
		},
		{
			name:        "secret present in the resolved vault -> green",
			secretNames: []string{"workload-db-password"},
			get: func(_ context.Context, _, _ string) (azsecrets.GetSecretResponse, error) {
				v := "x"
				return azsecrets.GetSecretResponse{Secret: azsecrets.Secret{Value: &v}}, nil
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name:        "secret missing / get error -> red",
			secretNames: []string{"workload-db-password"},
			get: func(_ context.Context, _, _ string) (azsecrets.GetSecretResponse, error) {
				return azsecrets.GetSecretResponse{}, errors.New("SecretNotFound")
			},
			wantStatus: preflight.StatusRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				KeyVaultURI: "https://demo.vault.azure.net/",
				SecretNames: tt.secretNames,
				Secrets:     stubSecrets{get: tt.get},
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
