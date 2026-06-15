package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// statusByID finds the result with the given stable ID.
func statusByID(res []cloud.CheckResult, id string) (preflight.Status, bool) {
	for _, r := range res {
		if r.ID == id {
			return r.Status, true
		}
	}
	return "", false
}

func TestCheckEgress(t *testing.T) {
	availVPC := func(_ context.Context, _ *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
		return &ec2.DescribeVpcsOutput{Vpcs: []ec2types.Vpc{{State: ec2types.VpcStateAvailable}}}, nil
	}
	availNAT := func(_ context.Context, _ *ec2.DescribeNatGatewaysInput) (*ec2.DescribeNatGatewaysOutput, error) {
		return &ec2.DescribeNatGatewaysOutput{NatGateways: []ec2types.NatGateway{{State: ec2types.NatGatewayStateAvailable}}}, nil
	}
	// A route table whose default route targets a firewall VPC endpoint (in-path)
	// and which does NOT open the IMDS CIDR.
	fwRoutes := func(_ context.Context, _ *ec2.DescribeRouteTablesInput) (*ec2.DescribeRouteTablesOutput, error) {
		zero := "0.0.0.0/0"
		vpce := "vpce-fw00000001"
		return &ec2.DescribeRouteTablesOutput{RouteTables: []ec2types.RouteTable{
			{Routes: []ec2types.Route{{DestinationCidrBlock: &zero, GatewayId: &vpce}}},
		}}, nil
	}

	tests := []struct {
		name       string
		vpcID      string // "" = default "vpc-123"; "-" = empty (no VPC context)
		vpcs       func(context.Context, *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
		nats       func(context.Context, *ec2.DescribeNatGatewaysInput) (*ec2.DescribeNatGatewaysOutput, error)
		routes     func(context.Context, *ec2.DescribeRouteTablesInput) (*ec2.DescribeRouteTablesOutput, error)
		egressRef  string
		checkID    string
		wantStatus preflight.Status
	}{
		{
			name: "vpc available -> egress.vpc green", vpcs: availVPC, nats: availNAT, routes: fwRoutes,
			egressRef: "arn:aws:network-firewall:us-east-1:111122223333:firewall/x",
			checkID:   "egress.vpc", wantStatus: preflight.StatusGreen,
		},
		{
			name: "available NAT -> egress.nat green", vpcs: availVPC, nats: availNAT, routes: fwRoutes,
			egressRef: "arn:aws:network-firewall:us-east-1:111122223333:firewall/x",
			checkID:   "egress.nat", wantStatus: preflight.StatusGreen,
		},
		{
			name: "no NAT -> egress.nat amber", vpcs: availVPC,
			nats: func(_ context.Context, _ *ec2.DescribeNatGatewaysInput) (*ec2.DescribeNatGatewaysOutput, error) {
				return &ec2.DescribeNatGatewaysOutput{NatGateways: nil}, nil
			},
			routes: fwRoutes, egressRef: "arn:aws:network-firewall:us-east-1:111122223333:firewall/x",
			checkID: "egress.nat", wantStatus: preflight.StatusAmber,
		},
		{
			name: "default route via firewall endpoint -> egress.firewall_inpath green",
			vpcs: availVPC, nats: availNAT, routes: fwRoutes,
			egressRef: "arn:aws:network-firewall:us-east-1:111122223333:firewall/x",
			checkID:   "egress.firewall_inpath", wantStatus: preflight.StatusGreen,
		},
		{
			name: "no firewall route -> egress.firewall_inpath red",
			vpcs: availVPC, nats: availNAT,
			routes: func(_ context.Context, _ *ec2.DescribeRouteTablesInput) (*ec2.DescribeRouteTablesOutput, error) {
				return &ec2.DescribeRouteTablesOutput{RouteTables: nil}, nil
			},
			egressRef: "arn:aws:network-firewall:us-east-1:111122223333:firewall/x",
			checkID:   "egress.firewall_inpath", wantStatus: preflight.StatusRed,
		},
		{
			name: "empty egress_path_ref (BYO) -> egress.firewall_inpath amber",
			vpcs: availVPC, nats: availNAT, routes: fwRoutes, egressRef: "",
			checkID: "egress.firewall_inpath", wantStatus: preflight.StatusAmber,
		},
		{
			name: "no IMDS route -> egress.metadata_block green",
			vpcs: availVPC, nats: availNAT, routes: fwRoutes,
			egressRef: "arn:aws:network-firewall:us-east-1:111122223333:firewall/x",
			checkID:   "egress.metadata_block", wantStatus: preflight.StatusGreen,
		},
		{
			name: "vpc not found -> egress.vpc red",
			vpcs: func(_ context.Context, _ *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
				return &ec2.DescribeVpcsOutput{Vpcs: nil}, nil
			},
			checkID: "egress.vpc", wantStatus: preflight.StatusRed,
		},
		{
			name: "describe error -> egress.vpc red",
			vpcs: func(_ context.Context, _ *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
				return nil, errors.New("AccessDenied")
			},
			checkID: "egress.vpc", wantStatus: preflight.StatusRed,
		},
		{
			// No VPC context: egress posture is not applicable, not a failure.
			name: "empty VPC id -> egress.vpc amber (skipped)", vpcID: "-",
			checkID: "egress.vpc", wantStatus: preflight.StatusAmber,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// vpcID defaults to "vpc-123"; the sentinel "-" means test the empty case.
			vpcID := "vpc-123"
			if tt.vpcID == "-" {
				vpcID = ""
			} else if tt.vpcID != "" {
				vpcID = tt.vpcID
			}
			p := &Provider{
				Region:        "us-east-1",
				VPCID:         vpcID,
				EgressPathRef: tt.egressRef,
				EC2:           stubEC2{vpcs: tt.vpcs, nats: tt.nats, routes: tt.routes},
			}
			res := p.CheckEgress(context.Background())
			if len(res) == 0 {
				t.Fatal("expected a result")
			}
			got, ok := statusByID(res, tt.checkID)
			if !ok {
				t.Fatalf("no result with stable ID %q (got %d results)", tt.checkID, len(res))
			}
			if got != tt.wantStatus {
				t.Errorf("%s status = %q, want %q", tt.checkID, got, tt.wantStatus)
			}
		})
	}
}
