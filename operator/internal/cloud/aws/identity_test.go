package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func evalResult(action string, decision iamtypes.PolicyEvaluationDecisionType) iamtypes.EvaluationResult {
	a := action
	return iamtypes.EvaluationResult{EvalActionName: &a, EvalDecision: decision}
}

func TestCheckIdentityPermissions(t *testing.T) {
	allAllowed := func(_ context.Context, _ *iam.SimulatePrincipalPolicyInput) (*iam.SimulatePrincipalPolicyOutput, error) {
		return &iam.SimulatePrincipalPolicyOutput{EvaluationResults: []iamtypes.EvaluationResult{
			evalResult("kms:CreateKey", iamtypes.PolicyEvaluationDecisionTypeAllowed),
			evalResult("secretsmanager:CreateSecret", iamtypes.PolicyEvaluationDecisionTypeAllowed),
			evalResult("iam:CreateRole", iamtypes.PolicyEvaluationDecisionTypeAllowed),
			evalResult("eks:CreateCluster", iamtypes.PolicyEvaluationDecisionTypeAllowed),
		}}, nil
	}

	tests := []struct {
		name       string
		caller     func(context.Context, *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
		simulate   func(context.Context, *iam.SimulatePrincipalPolicyInput) (*iam.SimulatePrincipalPolicyOutput, error)
		principal  string
		concerns   []string // nil = unset (all concerns); explicit subset scopes the probe
		wantStatus preflight.Status
	}{
		{
			name:       "all required actions allowed -> green",
			principal:  "arn:aws:iam::111122223333:role/deployer",
			simulate:   allAllowed,
			wantStatus: preflight.StatusGreen,
		},
		{
			name:      "a required action denied -> red",
			principal: "arn:aws:iam::111122223333:role/deployer",
			simulate: func(_ context.Context, _ *iam.SimulatePrincipalPolicyInput) (*iam.SimulatePrincipalPolicyOutput, error) {
				return &iam.SimulatePrincipalPolicyOutput{EvaluationResults: []iamtypes.EvaluationResult{
					evalResult("kms:CreateKey", iamtypes.PolicyEvaluationDecisionTypeImplicitDeny),
					evalResult("secretsmanager:CreateSecret", iamtypes.PolicyEvaluationDecisionTypeAllowed),
					evalResult("iam:CreateRole", iamtypes.PolicyEvaluationDecisionTypeAllowed),
					evalResult("eks:CreateCluster", iamtypes.PolicyEvaluationDecisionTypeAllowed),
				}}, nil
			},
			wantStatus: preflight.StatusRed,
		},
		{
			name: "principal resolved via STS when not supplied -> green",
			caller: func(_ context.Context, _ *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
				arn := "arn:aws:iam::111122223333:role/auto"
				return &sts.GetCallerIdentityOutput{Arn: &arn}, nil
			},
			simulate: func(_ context.Context, in *iam.SimulatePrincipalPolicyInput) (*iam.SimulatePrincipalPolicyOutput, error) {
				if in.PolicySourceArn == nil || *in.PolicySourceArn != "arn:aws:iam::111122223333:role/auto" {
					return nil, errors.New("principal not resolved from STS")
				}
				return allAllowed(context.Background(), in)
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name:      "simulate error -> red",
			principal: "arn:aws:iam::111122223333:role/deployer",
			simulate: func(_ context.Context, _ *iam.SimulatePrincipalPolicyInput) (*iam.SimulatePrincipalPolicyOutput, error) {
				return nil, errors.New("AccessDenied")
			},
			wantStatus: preflight.StatusRed,
		},
		{
			// Fully BYO: nothing provisioned, so no provisioning permission required.
			// "-" is the test sentinel for an explicit empty concern set.
			name:       "no concerns provisioned (fully BYO) -> amber (not applicable)",
			principal:  "arn:aws:iam::111122223333:role/deployer",
			concerns:   []string{"-"},
			wantStatus: preflight.StatusAmber,
		},
		{
			// Only the cluster is provisioned: the probe must simulate ONLY
			// eks:CreateCluster, so a deny on the others is irrelevant.
			name:      "cluster-only scope, only eks allowed -> green",
			principal: "arn:aws:iam::111122223333:role/deployer",
			concerns:  []string{"cluster"},
			simulate: func(_ context.Context, in *iam.SimulatePrincipalPolicyInput) (*iam.SimulatePrincipalPolicyOutput, error) {
				// The probe must request exactly eks:CreateCluster and nothing else.
				if len(in.ActionNames) != 1 || in.ActionNames[0] != "eks:CreateCluster" {
					return nil, errors.New("expected only eks:CreateCluster to be simulated")
				}
				return &iam.SimulatePrincipalPolicyOutput{EvaluationResults: []iamtypes.EvaluationResult{
					evalResult("eks:CreateCluster", iamtypes.PolicyEvaluationDecisionTypeAllowed),
				}}, nil
			},
			wantStatus: preflight.StatusGreen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Map the "-" sentinel to an explicit empty set (fully BYO, nothing to
			// provision); nil stays nil → unset → all concerns (greenfield default).
			concerns := tt.concerns
			if len(concerns) == 1 && concerns[0] == "-" {
				concerns = []string{}
			}
			p := &Provider{
				Region:             "us-east-1",
				AccountID:          "111122223333",
				DeployPrincipalARN: tt.principal,
				ProvisionConcerns:  concerns,
				IAM:                stubIAM{simulate: tt.simulate},
				STS:                stubSTS{caller: tt.caller},
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
