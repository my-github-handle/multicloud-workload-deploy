package gcp

import (
	"context"
	"fmt"

	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// CheckEgress implements Stage 3: the resolved VPC network exists and has a
// controlled egress path (a Cloud NAT on its Cloud Router). FQDN-allowlist
// reachability to the control-plane FQDN/ghcr.io is the perimeter firewall's job
// (provisioned in the network module); here we assert the egress *path* exists.
func (p *Provider) CheckEgress(ctx context.Context) []cloud.CheckResult {
	netName := lastSegment(p.NetworkSelfLink)
	_, err := p.Compute.GetNetwork(ctx, &computepb.GetNetworkRequest{Project: p.ProjectID, Network: netName})
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "egress.vpc",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("cannot get VPC network %s: %v", p.NetworkSelfLink, err),
			Remediation: "grant compute.networks.get and confirm the network self-link is correct",
		}}
	}

	if p.RouterName == "" {
		return []cloud.CheckResult{{
			ID:          "egress.nat",
			Status:      preflight.StatusAmber,
			Message:     "VPC resolves but no Cloud Router was supplied to confirm a Cloud NAT egress path",
			Remediation: "supply the Cloud Router name (network module's router), or confirm the customer's controlled egress path",
		}}
	}

	nats, err := p.Compute.ListRouterNats(ctx, p.ProjectID, p.Region, p.RouterName)
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "egress.nat",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("VPC resolves but Cloud NAT status unknown: %v", err),
			Remediation: "grant compute.routers.get or confirm a controlled egress path exists",
		}}
	}
	if len(nats) == 0 {
		return []cloud.CheckResult{{
			ID:          "egress.nat",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("VPC %s has no Cloud NAT on router %s; controlled egress path not confirmed", p.NetworkSelfLink, p.RouterName),
			Remediation: "provision a Cloud NAT (network module) or confirm the customer's controlled egress path",
		}}
	}

	return []cloud.CheckResult{{
		ID:      "egress.vpc",
		Status:  preflight.StatusGreen,
		Message: fmt.Sprintf("VPC %s resolves with a controlled egress path (Cloud NAT)", p.NetworkSelfLink),
	}}
}

func lastSegment(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return s[i+1:]
		}
	}
	return s
}
