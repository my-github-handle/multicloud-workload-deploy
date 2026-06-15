// Package azure implements the cloud.PreflightProvider checks for Azure using the
// Azure SDK for Go. Each Check* method returns []cloud.CheckResult and never an
// error — an inability to check is itself a red/amber CheckResult, so the report
// stays the single source of truth.
package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
)

// --- Narrow SDK client interfaces (one method per API we call) so tests can stub
//     them without a live Azure subscription. ---

// KeysAPI is the subset of the Key Vault keys client the KMS check uses.
type KeysAPI interface {
	GetKey(ctx context.Context, name, version string, opts *azkeys.GetKeyOptions) (azkeys.GetKeyResponse, error)
}

// SecretsAPI is the subset of the Key Vault secrets client the secrets check uses.
type SecretsAPI interface {
	GetSecret(ctx context.Context, name, version string, opts *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
}

// RoleAssignmentsAPI is the subset of armauthorization used by the identity
// check. ListForScope returns the role assignments at a scope (the resolved Key
// Vault) so the check can confirm the UAMI holds a non-privileged role there.
type RoleAssignmentsAPI interface {
	ListForScope(ctx context.Context, scope, filter string) ([]*armauthorization.RoleAssignment, error)
}

// VNetsAPI is the subset of armnetwork used by the egress check.
type VNetsAPI interface {
	Get(ctx context.Context, resourceGroup, vnetName string) (armnetwork.VirtualNetworksClientGetResponse, error)
}

// Provider implements cloud.PreflightProvider for Azure.
type Provider struct {
	SubscriptionID string

	Keys       KeysAPI
	Secrets    SecretsAPI
	RoleAssign RoleAssignmentsAPI
	VNets      VNetsAPI

	// Resolved resource references the checks scope against.
	KeyVaultURI string   // e.g. https://demo.vault.azure.net/
	KeyName     string   // key name within the vault
	KeyVaultID  string   // ARM resource ID of the vault (role-assignment scope)
	SecretNames []string // secret names within the vault
	VNetID      string   // ARM resource ID of the resolved VNet

	// UAMIPrincipalID is the principal (object) ID whose role assignments Stage 0
	// inspects at the Key Vault scope.
	UAMIPrincipalID string
}

// Options configures New.
type Options struct {
	SubscriptionID  string
	KeyVaultURI     string
	KeyName         string
	KeyVaultID      string
	SecretNames     []string
	VNetID          string
	UAMIPrincipalID string
}

// New builds a Provider with real Azure SDK clients from DefaultAzureCredential
// (env, workload identity, managed identity, az CLI). Used by the preflight
// binary's selectProvider("azure").
func New(_ context.Context, opts Options) (*Provider, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}

	keysClient, err := azkeys.NewClient(opts.KeyVaultURI, cred, nil)
	if err != nil {
		return nil, err
	}
	secretsClient, err := azsecrets.NewClient(opts.KeyVaultURI, cred, nil)
	if err != nil {
		return nil, err
	}
	raClient, err := armauthorization.NewRoleAssignmentsClient(opts.SubscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	vnetClient, err := armnetwork.NewVirtualNetworksClient(opts.SubscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}

	return &Provider{
		SubscriptionID:  opts.SubscriptionID,
		Keys:            keysClient,
		Secrets:         secretsClient,
		RoleAssign:      roleAssignAdapter{raClient},
		VNets:           vnetAdapter{vnetClient},
		KeyVaultURI:     opts.KeyVaultURI,
		KeyName:         opts.KeyName,
		KeyVaultID:      opts.KeyVaultID,
		SecretNames:     opts.SecretNames,
		VNetID:          opts.VNetID,
		UAMIPrincipalID: opts.UAMIPrincipalID,
	}, nil
}

// roleAssignAdapter drains the role-assignments pager into a slice so the real
// client satisfies the slice-returning RoleAssignmentsAPI interface.
type roleAssignAdapter struct {
	c *armauthorization.RoleAssignmentsClient
}

func (a roleAssignAdapter) ListForScope(ctx context.Context, scope, filter string) ([]*armauthorization.RoleAssignment, error) {
	var opts *armauthorization.RoleAssignmentsClientListForScopeOptions
	if filter != "" {
		opts = &armauthorization.RoleAssignmentsClientListForScopeOptions{Filter: &filter}
	}
	pager := a.c.NewListForScopePager(scope, opts)
	var out []*armauthorization.RoleAssignment
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Value...)
	}
	return out, nil
}

// vnetAdapter wraps the real VNet client's options-taking Get to satisfy VNetsAPI.
type vnetAdapter struct {
	c *armnetwork.VirtualNetworksClient
}

func (a vnetAdapter) Get(ctx context.Context, resourceGroup, vnetName string) (armnetwork.VirtualNetworksClientGetResponse, error) {
	return a.c.Get(ctx, resourceGroup, vnetName, nil)
}

var _ cloud.PreflightProvider = (*Provider)(nil)
