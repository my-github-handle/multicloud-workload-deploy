package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

// rgAndName splits an ARM VNet resource ID into (resourceGroup, vnetName).
func rgAndName(vnetID string) (string, string) {
	parts := strings.Split(vnetID, "/")
	var rg, name string
	for i, p := range parts {
		if strings.EqualFold(p, "resourceGroups") && i+1 < len(parts) {
			rg = parts[i+1]
		}
	}
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}
	return rg, name
}

// CheckEgress implements Stage 3: the resolved VNet exists and at least one
// subnet has a route table (the UDR forcing egress through the Azure Firewall =
// the controlled egress path). FQDN-allowlist reachability is the firewall's job;
// here we assert the egress path exists.
func (p *Provider) CheckEgress(ctx context.Context) []cloud.CheckResult {
	rg, name := rgAndName(p.VNetID)
	out, err := p.VNets.Get(ctx, rg, name)
	if err != nil {
		return []cloud.CheckResult{{
			ID:          "azure.egress.vnet",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("cannot get VNet %s: %v", p.VNetID, err),
			Remediation: "grant Microsoft.Network/virtualNetworks/read and confirm the VNet ID is correct",
		}}
	}
	if out.Properties == nil || len(out.Properties.Subnets) == 0 {
		return []cloud.CheckResult{{
			ID:          "azure.egress.vnet",
			Status:      preflight.StatusRed,
			Message:     fmt.Sprintf("VNet %s has no subnets", p.VNetID),
			Remediation: "supply a resolvable VNet with a node subnet (provision the network module or fix the BYO VNet)",
		}}
	}

	hasUDR := false
	for _, s := range out.Properties.Subnets {
		if s.Properties != nil && s.Properties.RouteTable != nil && s.Properties.RouteTable.ID != nil {
			hasUDR = true
			break
		}
	}
	if !hasUDR {
		return []cloud.CheckResult{{
			ID:          "azure.egress.udr",
			Status:      preflight.StatusAmber,
			Message:     fmt.Sprintf("VNet %s has no subnet route table; controlled egress path (UDR → firewall) not confirmed", p.VNetID),
			Remediation: "attach a route table forcing 0.0.0.0/0 through the Azure Firewall (network module), or confirm the customer's controlled egress path",
		}}
	}

	return []cloud.CheckResult{{
		ID:      "azure.egress",
		Status:  preflight.StatusGreen,
		Message: fmt.Sprintf("VNet %s present with a controlled egress path (UDR route table)", p.VNetID),
	}}
}
