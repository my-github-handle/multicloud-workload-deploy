package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// deployActionByConcern maps each provisionable concern to one keystone
// create-action. The deploy-identity probe simulates only the actions for the
// concerns being provisioned; a BYO concern contributes none. The full
// least-privilege deploy policy is the artifact the iam module renders
// (modules/aws/iam deploy-policy.json); each action here also appears there.
var deployActionByConcern = map[string]string{
	"kms":     "kms:CreateKey",
	"secrets": "secretsmanager:CreateSecret",
	"iam":     "iam:CreateRole",
	"cluster": "eks:CreateCluster",
}

// concernOrder fixes the simulated-action order so the report is deterministic.
var concernOrder = []string{"kms", "secrets", "iam", "cluster"}

// requiredDeployActions returns the keystone actions to simulate for the concerns
// being provisioned, in concernOrder.
func (p *Provider) requiredDeployActions() []string {
	concerns := p.provisionConcerns()
	var actions []string
	for _, c := range concernOrder {
		if concerns[c] {
			if a, ok := deployActionByConcern[c]; ok {
				actions = append(actions, a)
			}
		}
	}
	return actions
}

// provisionConcerns is the set of concerns being provisioned. A nil
// ProvisionConcerns means all concerns (greenfield); a non-nil empty slice means
// none (fully BYO).
func (p *Provider) provisionConcerns() map[string]bool {
	out := map[string]bool{}
	if p.ProvisionConcerns == nil {
		for _, c := range concernOrder {
			out[c] = true
		}
		return out
	}
	for _, c := range p.ProvisionConcerns {
		out[c] = true
	}
	return out
}

// CheckIdentityPermissions implements Stage 0: the deploy identity can provision
// the in-scope concerns (missing → red). With nothing being provisioned it is not
// applicable (amber). Excess-permission detection is out of scope for the simulate
// API.
func (p *Provider) CheckIdentityPermissions(ctx context.Context) []cloud.CheckResult {
	actions := p.requiredDeployActions()
	if len(actions) == 0 {
		return []cloud.CheckResult{{
			ID:          "aws.identity",
			Status:      preflight.StatusAmber,
			Message:     "no concerns are being provisioned (fully BYO); deploy-identity provisioning permissions not applicable",
			Remediation: "the workload's RUNTIME identity binding is still validated by the Kubernetes stages; no deploy-time provisioning grant is needed here",
		}}
	}

	principal := p.DeployPrincipalARN
	if principal == "" {
		id, err := p.STS.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil || id.Arn == nil {
			return []cloud.CheckResult{{
				ID:          "aws.identity.caller",
				Status:      preflight.StatusRed,
				Message:     fmt.Sprintf("could not resolve the deploy identity via STS: %v", err),
				Remediation: "ensure valid AWS credentials are present (env / shared config / IRSA)",
			}}
		}
		principal = *id.Arn
	}

	out, err := p.IAM.SimulatePrincipalPolicy(ctx, &iam.SimulatePrincipalPolicyInput{
		PolicySourceArn: ptr(principal),
		ActionNames:     actions,
	})
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "aws.identity.simulate",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("could not simulate deploy-identity permissions: %v", err),
			Remediation: "grant iam:SimulatePrincipalPolicy to the preflight identity, or review the policy manually",
		}}
	}

	var denied []string
	for _, r := range out.EvaluationResults {
		if r.EvalDecision != iamtypes.PolicyEvaluationDecisionTypeAllowed {
			if r.EvalActionName != nil {
				denied = append(denied, *r.EvalActionName)
			}
		}
	}
	if len(denied) > 0 {
		return []cloud.CheckResult{{
			ID:          "aws.identity.missing",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("deploy identity %s is missing required actions: %s", principal, strings.Join(denied, ", ")),
			Remediation: "attach the deploy-time policy artifact (modules/aws/iam deploy-policy.json) to the deploy identity",
		}}
	}

	return []cloud.CheckResult{{
		ID:      "aws.identity",
		Status:  preflight.StatusGreen,
		Message: fmt.Sprintf("deploy identity %s holds the probed keystone deploy actions; attach the full deploy-policy.json for the complete least-privilege set (excess-permission detection is best-effort)", principal),
	}}
}
