package gcp

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func TestCheckSecretsBackend(t *testing.T) {
	const cmk = "projects/demo/locations/us-central1/keyRings/r/cryptoKeys/k"
	tests := []struct {
		name       string
		getSecret  func(context.Context, *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error)
		secretIDs  []string
		wantStatus preflight.Status
	}{
		{
			name:       "no secrets configured -> amber",
			secretIDs:  nil,
			wantStatus: preflight.StatusAmber,
		},
		{
			name:      "secret CMEK-encrypted with resolved key -> green",
			secretIDs: []string{"projects/demo/secrets/demo-db"},
			getSecret: func(_ context.Context, _ *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
				return &secretmanagerpb.Secret{
					Replication: &secretmanagerpb.Replication{
						Replication: &secretmanagerpb.Replication_UserManaged_{
							UserManaged: &secretmanagerpb.Replication_UserManaged{
								Replicas: []*secretmanagerpb.Replication_UserManaged_Replica{
									{CustomerManagedEncryption: &secretmanagerpb.CustomerManagedEncryption{KmsKeyName: cmk}},
								},
							},
						},
					},
				}, nil
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name:      "secret CMEK-encrypted with a DIFFERENT key -> red",
			secretIDs: []string{"projects/demo/secrets/demo-db"},
			getSecret: func(_ context.Context, _ *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
				return &secretmanagerpb.Secret{
					Replication: &secretmanagerpb.Replication{
						Replication: &secretmanagerpb.Replication_UserManaged_{
							UserManaged: &secretmanagerpb.Replication_UserManaged{
								Replicas: []*secretmanagerpb.Replication_UserManaged_Replica{
									{CustomerManagedEncryption: &secretmanagerpb.CustomerManagedEncryption{KmsKeyName: "projects/demo/locations/us-central1/keyRings/r/cryptoKeys/OTHER"}},
								},
							},
						},
					},
				}, nil
			},
			wantStatus: preflight.StatusRed,
		},
		{
			name:      "get error -> red",
			secretIDs: []string{"projects/demo/secrets/demo-db"},
			getSecret: func(_ context.Context, _ *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
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
				KMSKeyID:  cmk,
				SecretIDs: tt.secretIDs,
				Secrets:   stubSecrets{getSecret: tt.getSecret},
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
