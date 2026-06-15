output "vnet_id" {
  description = "Provisioned VNet resource ID."
  value       = azurerm_virtual_network.this.id
}

output "node_subnet_id" {
  description = "AKS node subnet ID (no public IPs; egress forced through the firewall)."
  value       = azurerm_subnet.nodes.id
}

output "private_subnet_ids" {
  description = "Private (node) subnet IDs — list shape so the resolver re-exports it uniformly with AWS/GCP."
  value       = [azurerm_subnet.nodes.id]
}

output "egress_path_ref" {
  description = "Reference to the controlled egress path (the Azure Firewall ID). The resolver re-exports this uniformly."
  value       = azurerm_firewall.this.id
}

output "egress_public_ip" {
  description = "The Azure Firewall's public (SNAT) IP — the source IP node/pod egress is seen as. For a public API endpoint with userDefinedRouting, this must be in the cluster's authorized IP ranges so nodes can reach the API server."
  value       = azurerm_public_ip.firewall.ip_address
}

output "flow_log_storage_id" {
  description = "ID of the customer-owned, immutable Storage Account holding VNet flow logs (the always-on audit floor)."
  value       = azurerm_storage_account.flow_logs.id
}
