// Package gcp implements the cloud.PreflightProvider preflight checks for GCP
// using the cloud.google.com/go/* SDKs and google.golang.org/api. Each Check*
// method returns []cloud.CheckResult and never an error — an inability to check
// is itself a red/amber CheckResult, so the report stays the single source of
// truth.
package gcp

import (
	"context"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"golang.org/x/oauth2/google"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
)

// --- Narrow SDK client interfaces (one method per API we call) so tests can
//     stub them without a live GCP project. ---

// IAMAPI is the subset used by the identity check. We use the Cloud Resource
// Manager TestIamPermissions on the project resource to confirm the deploy
// identity holds the required permissions (the GCP analogue of AWS
// SimulatePrincipalPolicy).
type IAMAPI interface {
	TestProjectIamPermissions(ctx context.Context, project string, perms []string) ([]string, error)
}

// KMSAPI is the subset of the KMS client the key check uses.
type KMSAPI interface {
	GetCryptoKey(ctx context.Context, in *kmspb.GetCryptoKeyRequest) (*kmspb.CryptoKey, error)
	GetCryptoKeyVersion(ctx context.Context, in *kmspb.GetCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error)
}

// SecretsAPI is the subset of the Secret Manager client the secrets check uses.
type SecretsAPI interface {
	GetSecret(ctx context.Context, in *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error)
}

// ComputeAPI is the subset of the Compute clients the egress check uses.
type ComputeAPI interface {
	GetNetwork(ctx context.Context, in *computepb.GetNetworkRequest) (*computepb.Network, error)
	ListRouterNats(ctx context.Context, project, region, router string) ([]*computepb.RouterNat, error)
}

// Provider implements cloud.PreflightProvider for GCP.
type Provider struct {
	ProjectID string
	Region    string

	IAM     IAMAPI
	KMS     KMSAPI
	Secrets SecretsAPI
	Compute ComputeAPI

	// Resolved resource references the checks scope against.
	KMSKeyID        string   // projects/P/locations/L/keyRings/R/cryptoKeys/K
	SecretIDs       []string // projects/P/secrets/S
	NetworkSelfLink string
	RouterName      string // Cloud Router whose NATs prove a controlled egress path.
}

// Options configures New.
type Options struct {
	ProjectID       string
	Region          string
	KMSKeyID        string
	SecretIDs       []string
	NetworkSelfLink string
	RouterName      string
}

// crmIAM adapts the Cloud Resource Manager service to IAMAPI.
type crmIAM struct{ svc *cloudresourcemanager.Service }

func (c crmIAM) TestProjectIamPermissions(ctx context.Context, project string, perms []string) ([]string, error) {
	resp, err := c.svc.Projects.TestIamPermissions(project, &cloudresourcemanager.TestIamPermissionsRequest{Permissions: perms}).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return resp.Permissions, nil
}

// clientOptions returns the SDK client options to build with. When application
// default credentials are present, it returns none (normal ADC auth). When they
// are absent, it returns WithoutAuthentication so the clients still CONSTRUCT —
// the provider is then usable but unauthenticated, and the staged checks fail
// into red CheckResults on the first real API call rather than New() erroring.
// This keeps construction credential-free (e.g. in CI) while preserving the
// report-is-the-single-source-of-truth contract.
func clientOptions(ctx context.Context) []option.ClientOption {
	if _, err := google.FindDefaultCredentials(ctx); err != nil {
		return []option.ClientOption{option.WithoutAuthentication()}
	}
	return nil
}

// New builds a Provider with real SDK clients. It uses application default
// credentials (ADC) when present; without them the clients are built
// unauthenticated so construction never fails (real calls then surface as red
// CheckResults). Used by the preflight binary's selectProvider("gcp").
func New(ctx context.Context, opts Options) (*Provider, error) {
	co := clientOptions(ctx)

	kmsCli, err := kms.NewKeyManagementClient(ctx, co...)
	if err != nil {
		return nil, err
	}
	smCli, err := secretmanager.NewClient(ctx, co...)
	if err != nil {
		return nil, err
	}
	netCli, err := compute.NewNetworksRESTClient(ctx, co...)
	if err != nil {
		return nil, err
	}
	natCli, err := compute.NewRoutersRESTClient(ctx, co...)
	if err != nil {
		return nil, err
	}
	crm, err := cloudresourcemanager.NewService(ctx, co...)
	if err != nil {
		return nil, err
	}

	p := &Provider{
		ProjectID:       opts.ProjectID,
		Region:          opts.Region,
		IAM:             crmIAM{svc: crm},
		KMS:             kmsAdapter{cli: kmsCli},
		Secrets:         secretsAdapter{cli: smCli},
		Compute:         computeAdapter{net: netCli, nat: natCli},
		KMSKeyID:        opts.KMSKeyID,
		SecretIDs:       opts.SecretIDs,
		NetworkSelfLink: opts.NetworkSelfLink,
		RouterName:      opts.RouterName,
	}
	return p, nil
}

// kmsAdapter / secretsAdapter / computeAdapter wrap the concrete SDK clients to
// satisfy the narrow interfaces above.
type kmsAdapter struct{ cli *kms.KeyManagementClient }

func (a kmsAdapter) GetCryptoKey(ctx context.Context, in *kmspb.GetCryptoKeyRequest) (*kmspb.CryptoKey, error) {
	return a.cli.GetCryptoKey(ctx, in)
}
func (a kmsAdapter) GetCryptoKeyVersion(ctx context.Context, in *kmspb.GetCryptoKeyVersionRequest) (*kmspb.CryptoKeyVersion, error) {
	return a.cli.GetCryptoKeyVersion(ctx, in)
}

type secretsAdapter struct{ cli *secretmanager.Client }

func (a secretsAdapter) GetSecret(ctx context.Context, in *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
	return a.cli.GetSecret(ctx, in)
}

type computeAdapter struct {
	net *compute.NetworksClient
	nat *compute.RoutersClient
}

func (a computeAdapter) GetNetwork(ctx context.Context, in *computepb.GetNetworkRequest) (*computepb.Network, error) {
	return a.net.Get(ctx, in)
}
func (a computeAdapter) ListRouterNats(ctx context.Context, project, region, router string) ([]*computepb.RouterNat, error) {
	r, err := a.nat.Get(ctx, &computepb.GetRouterRequest{Project: project, Region: region, Router: router})
	if err != nil {
		return nil, err
	}
	return r.GetNats(), nil
}

var _ cloud.PreflightProvider = (*Provider)(nil)
