package gcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// requiredDeployPermissions is the keystone create-permission set for the
// gcp-full path (KMS, Secret Manager, custom role + SA, GKE, network).
//
// Authority + anti-drift: the DEPLOY-TIME least-privilege artifact rendered by
// modules/gcp/iam (deploy-policy/role.json) is the reviewable contract shipped
// alongside the runtime custom-role.json. This Go slice MUST stay in lockstep
// with that artifact's permission list — the iam golden test
// (tests/no_wildcards.tftest.hcl) asserts every permission here appears in the
// rendered deploy-time role, so the probe and the artifact cannot drift. This
// slice doubles as the preflight PROBE: testIamPermissions on these keystone
// permissions cheaply answers "does the deploy identity plausibly hold the
// deploy grants?". No primitive roles are ever requested.
//
// KNOWN LIMITATION (documented, not silent): the artifact lists the full deploy
// set while this probe checks the keystone subset; the golden test enforces the
// subset relationship. A fully shared single manifest is a deferred refactor —
// until then the golden-test assertion is the drift guard.
var requiredDeployPermissions = []string{
	"cloudkms.keyRings.create",
	"cloudkms.cryptoKeys.create",
	"secretmanager.secrets.create",
	"iam.roles.create",
	"iam.serviceAccounts.create",
	"container.clusters.create",
	"compute.networks.create",
}

// CheckIdentityPermissions implements Stage 0: the deploy identity holds the
// least-privilege permissions the path needs — flags missing (red). Excess
// detection (amber) is best-effort and out of scope for testIamPermissions,
// noted in the green message.
func (p *Provider) CheckIdentityPermissions(ctx context.Context) []cloud.CheckResult {
	held, err := p.IAM.TestProjectIamPermissions(ctx, p.ProjectID, requiredDeployPermissions)
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "gcp.identity.test",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("could not test deploy-identity permissions on project %s: %v", p.ProjectID, err),
			Remediation: "ensure valid GCP application default credentials are present and can call resourcemanager.projects.testIamPermissions",
		}}
	}

	heldSet := map[string]bool{}
	for _, perm := range held {
		heldSet[perm] = true
	}
	var missing []string
	for _, req := range requiredDeployPermissions {
		if !heldSet[req] {
			missing = append(missing, req)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return []cloud.CheckResult{{
			ID:          "gcp.identity.missing",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("deploy identity is missing required permissions on project %s: %s", p.ProjectID, strings.Join(missing, ", ")),
			Remediation: "grant the deploy-time custom role (modules/gcp/iam deploy-policy/role.json) to the deploy identity",
		}}
	}

	return []cloud.CheckResult{{
		ID:      "gcp.identity",
		Status:  preflight.StatusGreen,
		Message: fmt.Sprintf("deploy identity holds all required permissions on project %s (excess-permission detection is best-effort)", p.ProjectID),
	}}
}
