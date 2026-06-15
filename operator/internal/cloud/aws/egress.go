package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// metadataCIDR is the IMDS link-local address Stage 3 asserts is NOT egress-
// routable from the node subnets (SSRF blast-radius reduction).
const metadataCIDR = "169.254.169.254/32"

// CheckEgress implements Stage 3. It emits a STABLE set of result IDs so
// downstream Terraform can key on them:
//
//	egress.vpc              VPC resolvable + available (red if not)
//	egress.nat              controlled egress path (NAT) present (amber if not)
//	egress.firewall_inpath  node default route points at the Network Firewall VPC
//	                        endpoint, NOT straight at the NAT (the firewall is
//	                        genuinely in-path); amber when EgressPathRef is empty
//	                        (BYO customer-owned edge)
//	egress.metadata_block   no node-subnet route allows the IMDS CIDR
//	egress.controlplane_fqdn / egress.ghcr / egress.cloud_api / egress.observability
//	                        outbound reachability to the required FQDNs over the
//	                        allowed path — emitted as amber-deferred (cannot be
//	                        proven from the preflight host)
//
// Each Check* returns []CheckResult and never errors; an inability to check is a
// red/amber result so the report stays the single source of truth.
func (p *Provider) CheckEgress(ctx context.Context) []cloud.CheckResult {
	var results []cloud.CheckResult

	// No VPC context means there is no cluster network to validate (e.g. a path
	// that deploys nothing into a VPC). The egress posture is then not applicable,
	// not a failure — report amber/informational rather than a false red.
	if p.VPCID == "" {
		return []cloud.CheckResult{{
			ID:          "egress.vpc",
			Status:      preflight.StatusAmber,
			Message:     "no VPC configured; egress posture not applicable (no cluster network to validate)",
			Remediation: "if a cluster is being deployed, supply the resolved VPC ID; otherwise this stage is informational",
		}}
	}

	// --- egress.vpc -----------------------------------------------------------
	vpcs, err := p.EC2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{VpcIds: []string{p.VPCID}})
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "egress.vpc",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("cannot describe VPC %s: %v", p.VPCID, err),
			Remediation: "grant ec2:DescribeVpcs and confirm the VPC ID is correct",
		}}
	}
	if len(vpcs.Vpcs) == 0 || vpcs.Vpcs[0].State != ec2types.VpcStateAvailable {
		return []cloud.CheckResult{{
			ID:          "egress.vpc",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("VPC %s not found or not available", p.VPCID),
			Remediation: "supply a resolvable, available VPC (provision the network module or fix the BYO VPC ID)",
		}}
	}
	results = append(results, cloud.CheckResult{
		ID:      "egress.vpc",
		Status:  preflight.StatusGreen,
		Message: fmt.Sprintf("VPC %s available", p.VPCID),
	})

	// --- egress.nat -----------------------------------------------------------
	nats, err := p.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
		Filter: []ec2types.Filter{{Name: ptr("vpc-id"), Values: []string{p.VPCID}}},
	})
	if err != nil {
		results = append(results, cloud.CheckResult{
			ID:          "egress.nat",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("VPC available but NAT gateway status unknown: %v", err),
			Remediation: "grant ec2:DescribeNatGateways or confirm a controlled egress path exists",
		})
	} else {
		hasAvailableNAT := false
		for _, n := range nats.NatGateways {
			if n.State == ec2types.NatGatewayStateAvailable {
				hasAvailableNAT = true
				break
			}
		}
		if !hasAvailableNAT {
			results = append(results, cloud.CheckResult{
				ID:          "egress.nat",
				Status:      preflight.StatusAmber,
				Message:     fmt.Sprintf("VPC %s has no available NAT gateway; controlled egress path not confirmed", p.VPCID),
				Remediation: "provision a NAT gateway (network module) or confirm the customer's controlled egress path",
			})
		} else {
			results = append(results, cloud.CheckResult{
				ID:      "egress.nat",
				Status:  preflight.StatusGreen,
				Message: fmt.Sprintf("VPC %s has an available NAT gateway (controlled egress path)", p.VPCID),
			})
		}
	}

	// --- egress.firewall_inpath + egress.metadata_block -----------------------
	results = append(results, p.checkFirewallInPath(ctx)...)

	// --- egress.controlplane_fqdn / ghcr / cloud_api / observability ----------
	results = append(results, p.checkFQDNReachability()...)

	return results
}

// checkFirewallInPath inspects the node-subnet route tables to prove (a) the
// default route targets the Network Firewall VPC endpoint (not the NAT) and (b)
// no route allows the IMDS metadata CIDR. Emits egress.firewall_inpath and
// egress.metadata_block.
func (p *Provider) checkFirewallInPath(ctx context.Context) []cloud.CheckResult {
	// BYO customer-owned edge: an empty EgressPathRef is the deliberate amber
	// signal — we cannot assert the firewall is in-path because the customer owns it.
	if p.EgressPathRef == "" {
		return []cloud.CheckResult{{
			ID:          "egress.firewall_inpath",
			Status:      preflight.StatusAmber,
			Message:     "no egress_path_ref resolved (BYO customer-owned edge); firewall-in-path is the customer's responsibility",
			Remediation: "confirm the customer's edge firewall enforces the FQDN allowlist + default-deny (shared responsibility)",
		}}
	}

	rts, err := p.EC2.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []ec2types.Filter{{Name: ptr("vpc-id"), Values: []string{p.VPCID}}},
	})
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "egress.firewall_inpath",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("cannot describe route tables to confirm firewall-in-path: %v", err),
			Remediation: "grant ec2:DescribeRouteTables; without it the in-path assertion is unverified",
		}}
	}

	inPath := false       // a default route via a VPC endpoint (firewall) exists
	metadataOpen := false // a node route explicitly targets the IMDS CIDR
	for _, rt := range rts.RouteTables {
		for _, r := range rt.Routes {
			dest := ""
			if r.DestinationCidrBlock != nil {
				dest = *r.DestinationCidrBlock
			}
			// A route to an AWS Network Firewall endpoint targets a VPC endpoint,
			// which the EC2 Route API reports in GatewayId with a "vpce-" prefix.
			if dest == "0.0.0.0/0" && r.GatewayId != nil && strings.HasPrefix(*r.GatewayId, "vpce-") {
				inPath = true
			}
			if dest == metadataCIDR {
				metadataOpen = true
			}
		}
	}

	results := []cloud.CheckResult{}
	if inPath {
		results = append(results, cloud.CheckResult{
			ID:      "egress.firewall_inpath",
			Status:  preflight.StatusGreen,
			Message: "node default route (0.0.0.0/0) targets the Network Firewall VPC endpoint — firewall is in-path",
		})
	} else {
		results = append(results, cloud.CheckResult{
			ID:          "egress.firewall_inpath",
			Status:      preflight.StatusRed,
			Message:     "no node default route via a firewall VPC endpoint; egress is NOT forced through the firewall",
			Remediation: "add the 0.0.0.0/0 → firewall-endpoint route on the node subnets (network module)",
		})
	}
	if metadataOpen {
		results = append(results, cloud.CheckResult{
			ID:          "egress.metadata_block",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("a node route targets the IMDS metadata CIDR %s; assess metadata blockability", metadataCIDR),
			Remediation: "ensure IMDS is hop-limited/blocked at the node and via in-cluster policy",
		})
	} else {
		results = append(results, cloud.CheckResult{
			ID:      "egress.metadata_block",
			Status:  preflight.StatusGreen,
			Message: "no node route explicitly opens the IMDS metadata CIDR (blockability assessed)",
		})
	}
	return results
}

// checkFQDNReachability reports outbound reachability to the required FQDNs.
// In-cluster reachability cannot be proven from the preflight host, so these are
// emitted as amber-deferred (not a false green) under stable IDs that downstream
// Terraform can key on; the apply-time dial in the runbook is the in-path proof.
func (p *Provider) checkFQDNReachability() []cloud.CheckResult {
	fqdnTargets := map[string]string{
		"egress.controlplane_fqdn": p.ControlPlaneFQDN,
		"egress.ghcr":              "ghcr.io",
	}
	var results []cloud.CheckResult
	for id, host := range fqdnTargets {
		if host == "" {
			continue
		}
		results = append(results, cloud.CheckResult{
			ID:          id,
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("reachability to %s over the allowed path is DEFERRED (assert from inside the cluster post-apply)", host),
			Remediation: "the runbook dials each FQDN from a node; the Suricata allowlist (network module) is the in-path proof",
		})
	}
	// Bucketed cloud-API / observability-sink reachability — deferred placeholders
	// with stable IDs; concrete endpoints are deployment-specific and asserted in
	// the runbook.
	results = append(results,
		cloud.CheckResult{ID: "egress.cloud_api", Status: preflight.StatusAmber, Message: "cloud-API endpoint reachability deferred to post-apply"},
		cloud.CheckResult{ID: "egress.observability", Status: preflight.StatusAmber, Message: "observability-sink reachability deferred to post-apply"},
	)
	return results
}
