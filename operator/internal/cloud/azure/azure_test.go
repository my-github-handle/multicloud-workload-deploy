package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
)

// TestProviderSatisfiesInterface is a compile-time assertion that *Provider
// implements cloud.PreflightProvider (all four Check* methods).
func TestProviderSatisfiesInterface(t *testing.T) {
	var _ cloud.PreflightProvider = (*Provider)(nil)
}

// TestNewProviderWiresFields verifies a Provider built directly carries its
// resolved references.
func TestNewProviderWiresFields(t *testing.T) {
	p := &Provider{
		SubscriptionID:  "00000000-0000-0000-0000-000000000000",
		Keys:            stubKeys{},
		Secrets:         stubSecrets{},
		RoleAssign:      stubRoleAssign{},
		VNets:           stubVNets{},
		KeyVaultURI:     "https://demo.vault.azure.net/",
		KeyName:         "workload-cmk",
		SecretNames:     []string{"workload-db-password"},
		VNetID:          "/subscriptions/x/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet",
		UAMIPrincipalID: "11111111-1111-1111-1111-111111111111",
	}
	if p.SubscriptionID == "" {
		t.Fatalf("subscription not set")
	}
}

// --- Stub SDK clients. Return empty successful responses by default; per-method
//     tests override behavior via the function fields. ---

type stubKeys struct {
	get func(context.Context, string, string) (azkeys.GetKeyResponse, error)
}

func (s stubKeys) GetKey(ctx context.Context, name, version string, _ *azkeys.GetKeyOptions) (azkeys.GetKeyResponse, error) {
	if s.get != nil {
		return s.get(ctx, name, version)
	}
	return azkeys.GetKeyResponse{}, nil
}

type stubSecrets struct {
	get func(context.Context, string, string) (azsecrets.GetSecretResponse, error)
}

func (s stubSecrets) GetSecret(ctx context.Context, name, version string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
	if s.get != nil {
		return s.get(ctx, name, version)
	}
	return azsecrets.GetSecretResponse{}, nil
}

type stubRoleAssign struct {
	list func(context.Context, string, string) ([]*armauthorization.RoleAssignment, error)
}

func (s stubRoleAssign) ListForScope(ctx context.Context, scope, filter string) ([]*armauthorization.RoleAssignment, error) {
	if s.list != nil {
		return s.list(ctx, scope, filter)
	}
	return nil, nil
}

type stubVNets struct {
	get func(context.Context, string, string) (armnetwork.VirtualNetworksClientGetResponse, error)
}

func (s stubVNets) Get(ctx context.Context, rg, name string) (armnetwork.VirtualNetworksClientGetResponse, error) {
	if s.get != nil {
		return s.get(ctx, rg, name)
	}
	return armnetwork.VirtualNetworksClientGetResponse{}, nil
}
