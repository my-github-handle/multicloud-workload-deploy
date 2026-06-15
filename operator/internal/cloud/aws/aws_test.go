package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
)

// TestProviderSatisfiesInterface is a compile-time assertion that *Provider
// implements cloud.PreflightProvider (all four Check* methods).
func TestProviderSatisfiesInterface(t *testing.T) {
	var _ cloud.PreflightProvider = (*Provider)(nil)
}

// TestNewProviderWiresFields verifies the struct holds the resolved references
// the checks scope against.
func TestNewProviderWiresFields(t *testing.T) {
	p := &Provider{
		Region:    "us-east-1",
		AccountID: "111122223333",
		IAM:       stubIAM{},
		KMS:       stubKMS{},
		Secrets:   stubSecrets{},
		EC2:       stubEC2{},
		STS:       stubSTS{},
		KMSKeyARN: "arn:aws:kms:us-east-1:111122223333:key/abc",
		SecretARNs: []string{
			"arn:aws:secretsmanager:us-east-1:111122223333:secret:demo-db-AbCdEf",
		},
		VPCID: "vpc-123",
	}
	if p.Region != "us-east-1" {
		t.Fatalf("region not set")
	}
}

// --- SDK client stubs. Each method delegates to an optional function field so
//     per-stage tests override behavior; the default returns an empty success. ---

type stubIAM struct {
	simulate func(context.Context, *iam.SimulatePrincipalPolicyInput) (*iam.SimulatePrincipalPolicyOutput, error)
}

func (s stubIAM) SimulatePrincipalPolicy(ctx context.Context, in *iam.SimulatePrincipalPolicyInput, _ ...func(*iam.Options)) (*iam.SimulatePrincipalPolicyOutput, error) {
	if s.simulate != nil {
		return s.simulate(ctx, in)
	}
	return &iam.SimulatePrincipalPolicyOutput{}, nil
}

type stubKMS struct {
	describe func(context.Context, *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error)
	rotation func(context.Context, *kms.GetKeyRotationStatusInput) (*kms.GetKeyRotationStatusOutput, error)
}

func (s stubKMS) DescribeKey(ctx context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	if s.describe != nil {
		return s.describe(ctx, in)
	}
	return &kms.DescribeKeyOutput{}, nil
}

func (s stubKMS) GetKeyRotationStatus(ctx context.Context, in *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	if s.rotation != nil {
		return s.rotation(ctx, in)
	}
	return &kms.GetKeyRotationStatusOutput{}, nil
}

type stubSecrets struct {
	describe func(context.Context, *secretsmanager.DescribeSecretInput) (*secretsmanager.DescribeSecretOutput, error)
}

func (s stubSecrets) DescribeSecret(ctx context.Context, in *secretsmanager.DescribeSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	if s.describe != nil {
		return s.describe(ctx, in)
	}
	return &secretsmanager.DescribeSecretOutput{}, nil
}

type stubEC2 struct {
	vpcs   func(context.Context, *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	nats   func(context.Context, *ec2.DescribeNatGatewaysInput) (*ec2.DescribeNatGatewaysOutput, error)
	routes func(context.Context, *ec2.DescribeRouteTablesInput) (*ec2.DescribeRouteTablesOutput, error)
}

func (s stubEC2) DescribeVpcs(ctx context.Context, in *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	if s.vpcs != nil {
		return s.vpcs(ctx, in)
	}
	return &ec2.DescribeVpcsOutput{}, nil
}

func (s stubEC2) DescribeNatGateways(ctx context.Context, in *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	if s.nats != nil {
		return s.nats(ctx, in)
	}
	return &ec2.DescribeNatGatewaysOutput{}, nil
}

func (s stubEC2) DescribeRouteTables(ctx context.Context, in *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	if s.routes != nil {
		return s.routes(ctx, in)
	}
	return &ec2.DescribeRouteTablesOutput{}, nil
}

type stubSTS struct {
	caller func(context.Context, *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
}

func (s stubSTS) GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput, _ ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if s.caller != nil {
		return s.caller(ctx, in)
	}
	return &sts.GetCallerIdentityOutput{}, nil
}
