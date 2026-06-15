package gcp

import (
	"context"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
)

// TestProviderSatisfiesInterface is a compile-time assertion that *Provider
// implements cloud.PreflightProvider (all four Check* methods).
func TestProviderSatisfiesInterface(t *testing.T) {
	var _ cloud.PreflightProvider = (*Provider)(nil)
}

// TestNewProviderWiresClients verifies the Provider struct holds the supplied
// fields (the stub clients are wired via the struct, not New, so the test needs
// no live GCP).
func TestNewProviderWiresClients(t *testing.T) {
	p := &Provider{
		ProjectID: "demo-project",
		Region:    "us-central1",
		IAM:       stubIAM{},
		KMS:       stubKMS{},
		Secrets:   stubSecrets{},
		Compute:   stubCompute{},
		KMSKeyID:  "projects/demo-project/locations/us-central1/keyRings/r/cryptoKeys/k",
		SecretIDs: []string{
			"projects/demo-project/secrets/demo-db",
		},
		NetworkSelfLink: "https://www.googleapis.com/compute/v1/projects/demo-project/global/networks/demo-vpc",
	}
	if p.ProjectID != "demo-project" {
		t.Fatalf("project id not set")
	}
}

// --- Stubs for the narrow SDK client interfaces. Default behaviour is a
//     successful empty response; per-method tests override via the func fields. ---

type stubIAM struct {
	test func(context.Context, string, []string) ([]string, error)
}

func (s stubIAM) TestProjectIamPermissions(ctx context.Context, project string, perms []string) ([]string, error) {
	if s.test != nil {
		return s.test(ctx, project, perms)
	}
	return perms, nil // default: all requested permissions granted
}

type stubKMS struct {
	getKey func(context.Context, *kmspb.GetCryptoKeyRequest) (*kmspb.CryptoKey, error)
	getVer func(context.Context, *kmspb.GetCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error)
}

func (s stubKMS) GetCryptoKey(ctx context.Context, in *kmspb.GetCryptoKeyRequest) (*kmspb.CryptoKey, error) {
	if s.getKey != nil {
		return s.getKey(ctx, in)
	}
	return &kmspb.CryptoKey{}, nil
}
func (s stubKMS) GetCryptoKeyVersion(ctx context.Context, in *kmspb.GetCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error) {
	if s.getVer != nil {
		return s.getVer(ctx, in)
	}
	return &kmspb.CryptoKeyVersion{}, nil
}

type stubSecrets struct {
	getSecret func(context.Context, *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error)
}

func (s stubSecrets) GetSecret(ctx context.Context, in *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
	if s.getSecret != nil {
		return s.getSecret(ctx, in)
	}
	return &secretmanagerpb.Secret{}, nil
}

type stubCompute struct {
	getNet  func(context.Context, *computepb.GetNetworkRequest) (*computepb.Network, error)
	listNat func(context.Context, string, string, string) ([]*computepb.RouterNat, error)
}

func (s stubCompute) GetNetwork(ctx context.Context, in *computepb.GetNetworkRequest) (*computepb.Network, error) {
	if s.getNet != nil {
		return s.getNet(ctx, in)
	}
	return &computepb.Network{}, nil
}
func (s stubCompute) ListRouterNats(ctx context.Context, project, region, router string) ([]*computepb.RouterNat, error) {
	if s.listNat != nil {
		return s.listNat(ctx, project, region, router)
	}
	return nil, nil
}
