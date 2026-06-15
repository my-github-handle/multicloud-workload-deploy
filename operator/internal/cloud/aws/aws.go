// Package aws implements the cloud.PreflightProvider preflight checks for AWS using
// aws-sdk-go-v2. Each Check* method returns []cloud.CheckResult and never an error —
// an inability to check is itself a red/amber CheckResult, so the report stays the
// single source of truth.
package aws

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
)

// --- Narrow SDK client interfaces (one method per API we call) so tests can stub
//     them without a live AWS account. ---

// IAMAPI is the subset of the IAM client the identity check uses.
type IAMAPI interface {
	SimulatePrincipalPolicy(ctx context.Context, in *iam.SimulatePrincipalPolicyInput, optFns ...func(*iam.Options)) (*iam.SimulatePrincipalPolicyOutput, error)
}

// KMSAPI is the subset of the KMS client the key check uses.
type KMSAPI interface {
	DescribeKey(ctx context.Context, in *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	GetKeyRotationStatus(ctx context.Context, in *kms.GetKeyRotationStatusInput, optFns ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error)
}

// SecretsAPI is the subset of the Secrets Manager client the secrets check uses.
type SecretsAPI interface {
	DescribeSecret(ctx context.Context, in *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
}

// EC2API is the subset of the EC2 client the egress check uses.
type EC2API interface {
	DescribeVpcs(ctx context.Context, in *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeNatGateways(ctx context.Context, in *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
	// DescribeRouteTables backs the egress.firewall_inpath + egress.metadata_block
	// assertions: it inspects the node-subnet route tables to confirm the default
	// route points at the Network Firewall VPC endpoint (in-path) and that the
	// metadata IP (169.254.169.254/32) has no node-level allow route.
	DescribeRouteTables(ctx context.Context, in *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
}

// STSAPI is the subset of the STS client used to resolve the caller identity.
type STSAPI interface {
	GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// Provider implements cloud.PreflightProvider for AWS.
type Provider struct {
	Region    string
	AccountID string

	IAM     IAMAPI
	KMS     KMSAPI
	Secrets SecretsAPI
	EC2     EC2API
	STS     STSAPI

	// Resolved resource references the checks scope against.
	KMSKeyARN  string
	SecretARNs []string
	VPCID      string

	// Stage-3 egress inputs. EgressPathRef is the Network Firewall ARN from the
	// network-resolver (empty in BYO "customer-owned edge" mode → amber).
	// ControlPlaneFQDN / RequiredEgressFQDNs are the hosts whose reachability over
	// the allowed path Stage 3 reports on (ghcr.io, cloud APIs, observability
	// sinks, control-plane FQDN).
	EgressPathRef       string
	ControlPlaneFQDN    string
	RequiredEgressFQDNs []string

	// DeployPrincipalARN is the identity whose permissions Stage 0 simulates.
	// When empty the identity check resolves it via STS GetCallerIdentity.
	DeployPrincipalARN string

	// ProvisionConcerns lists the concerns being PROVISIONED (subset of
	// "kms","secrets","iam","cluster"); a BYO concern is omitted. Stage 0 simulates
	// only these concerns' create-actions. Empty/nil means "unset" → all concerns
	// (the greenfield default), preserving behavior when a caller does not scope it.
	ProvisionConcerns []string
}

// Options configures New.
type Options struct {
	Region              string
	KMSKeyARN           string
	SecretARNs          []string
	VPCID               string
	EgressPathRef       string
	ControlPlaneFQDN    string
	RequiredEgressFQDNs []string
	DeployPrincipalARN  string
	ProvisionConcerns   []string
}

// New builds a Provider with real aws-sdk-go-v2 clients from the default config
// chain (env, shared config, IRSA/instance profile). Used by the preflight
// binary's selectProvider("aws").
func New(ctx context.Context, opts Options) (*Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.Region))
	if err != nil {
		return nil, err
	}
	p := &Provider{
		Region:              opts.Region,
		IAM:                 iam.NewFromConfig(cfg),
		KMS:                 kms.NewFromConfig(cfg),
		Secrets:             secretsmanager.NewFromConfig(cfg),
		EC2:                 ec2.NewFromConfig(cfg),
		STS:                 sts.NewFromConfig(cfg),
		KMSKeyARN:           opts.KMSKeyARN,
		SecretARNs:          opts.SecretARNs,
		VPCID:               opts.VPCID,
		EgressPathRef:       opts.EgressPathRef,
		ControlPlaneFQDN:    opts.ControlPlaneFQDN,
		RequiredEgressFQDNs: opts.RequiredEgressFQDNs,
		DeployPrincipalARN:  opts.DeployPrincipalARN,
		ProvisionConcerns:   opts.ProvisionConcerns,
	}
	return p, nil
}

// ptr is a small helper for building *string inputs in the stage files.
func ptr(s string) *string { return awssdk.String(s) }

var _ cloud.PreflightProvider = (*Provider)(nil)
