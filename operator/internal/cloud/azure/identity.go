package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// builtInPrivilegedRoleGUIDs are the well-known built-in role definition GUIDs
// flagged as excess if the workload UAMI holds them (it should hold only the
// scoped custom role + AcrPull).
var builtInPrivilegedRoleGUIDs = map[string]string{
	"8e3af657-a8ff-443c-a75c-2fe8c4bcb635": "Owner",
	"b24988ac-6180-42a0-ab88-20f7382dd24c": "Contributor",
	"18d7d88d-d35e-4fb5-a5c3-7773c20a72d9": "User Access Administrator",
}

// CheckIdentityPermissions implements Stage 0: the workload UAMI holds the
// least-privilege custom role at the resolved Key Vault scope — flags a missing
// assignment (red) and a built-in privileged-role assignment as excess (amber,
// best-effort).
//
// This checks the runtime UAMI binding, not the deploy-time identity; deploy-time
// missing/excess detection is best-effort and deferred. The deploy-time policy is
// rendered as a reviewable artifact (modules/azure/iam deploy-policy.json) so it
// is inspectable even though this probe does not assert it.
func (p *Provider) CheckIdentityPermissions(ctx context.Context) []cloud.CheckResult {
	assignments, err := p.RoleAssign.ListForScope(ctx, p.KeyVaultID, "")
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "azure.identity.list",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("could not list role assignments at %s: %v", p.KeyVaultID, err),
			Remediation: "grant Microsoft.Authorization/roleAssignments/read to the preflight identity, or review the assignment manually",
		}}
	}

	var forUAMI []string
	for _, a := range assignments {
		if a == nil || a.Properties == nil || a.Properties.PrincipalID == nil || a.Properties.RoleDefinitionID == nil {
			continue
		}
		if !strings.EqualFold(*a.Properties.PrincipalID, p.UAMIPrincipalID) {
			continue
		}
		forUAMI = append(forUAMI, *a.Properties.RoleDefinitionID)
	}

	if len(forUAMI) == 0 {
		return []cloud.CheckResult{{
			ID:          "azure.identity.missing",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("workload UAMI %s has no role assignment at the resolved Key Vault %s", p.UAMIPrincipalID, p.KeyVaultID),
			Remediation: "assign the custom workload role to the UAMI scoped to the resolved Key Vault (modules/azure/iam role-definition.json)",
		}}
	}

	// Excess detection (best-effort): flag any built-in privileged role.
	for _, roleDefID := range forUAMI {
		guid := roleDefID[strings.LastIndex(roleDefID, "/")+1:]
		if name, ok := builtInPrivilegedRoleGUIDs[strings.ToLower(guid)]; ok {
			return []cloud.CheckResult{{
				ID:          "azure.identity.excess",
				Status:      preflight.StatusAmber,
				Message:     fmt.Sprintf("workload UAMI holds the built-in privileged role %q at %s — exceeds least privilege", name, p.KeyVaultID),
				Remediation: "replace the built-in privileged role with the scoped custom workload role (no Owner/Contributor)",
			}}
		}
	}

	return []cloud.CheckResult{{
		ID:      "azure.identity",
		Status:  preflight.StatusGreen,
		Message: fmt.Sprintf("workload UAMI holds a scoped (non-privileged) role at %s (excess-permission detection is best-effort)", p.KeyVaultID),
	}}
}
