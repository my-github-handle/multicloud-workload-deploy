package azure

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

const principal = "11111111-1111-1111-1111-111111111111"

func assignment(princ, roleDefID string) *armauthorization.RoleAssignment {
	return &armauthorization.RoleAssignment{
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID:      strp(princ),
			RoleDefinitionID: strp(roleDefID),
		},
	}
}

func TestCheckIdentityPermissions(t *testing.T) {
	const ownerRole = "/subscriptions/x/providers/Microsoft.Authorization/roleDefinitions/8e3af657-a8ff-443c-a75c-2fe8c4bcb635"
	const customRole = "/subscriptions/x/providers/Microsoft.Authorization/roleDefinitions/abcdabcd-0000-0000-0000-000000000000"

	tests := []struct {
		name       string
		list       func(context.Context, string, string) ([]*armauthorization.RoleAssignment, error)
		wantStatus preflight.Status
	}{
		{
			name: "UAMI holds the scoped custom role -> green",
			list: func(_ context.Context, _, _ string) ([]*armauthorization.RoleAssignment, error) {
				return []*armauthorization.RoleAssignment{assignment(principal, customRole)}, nil
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name: "UAMI holds a built-in privileged role (Owner) -> amber (excess)",
			list: func(_ context.Context, _, _ string) ([]*armauthorization.RoleAssignment, error) {
				return []*armauthorization.RoleAssignment{assignment(principal, ownerRole)}, nil
			},
			wantStatus: preflight.StatusAmber,
		},
		{
			name: "no assignment for the UAMI at the scope -> red",
			list: func(_ context.Context, _, _ string) ([]*armauthorization.RoleAssignment, error) {
				return []*armauthorization.RoleAssignment{assignment("other-principal", customRole)}, nil
			},
			wantStatus: preflight.StatusRed,
		},
		{
			name: "list error -> red",
			list: func(_ context.Context, _, _ string) ([]*armauthorization.RoleAssignment, error) {
				return nil, errors.New("AuthorizationFailed")
			},
			wantStatus: preflight.StatusRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				KeyVaultID:      "/subscriptions/x/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/demo",
				UAMIPrincipalID: principal,
				RoleAssign:      stubRoleAssign{list: tt.list},
			}
			res := p.CheckIdentityPermissions(context.Background())
			if len(res) == 0 {
				t.Fatal("expected a result")
			}
			if res[0].Status != tt.wantStatus {
				t.Errorf("status = %q, want %q (msg: %s)", res[0].Status, tt.wantStatus, res[0].Message)
			}
		})
	}
}
