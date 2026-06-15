output "network_self_link" {
  description = "Self-link of the provisioned VPC network (the GCP analogue of vpc_id)."
  value       = module.vpc.network_self_link
}

output "network_id" {
  description = "ID of the provisioned VPC network."
  value       = module.vpc.network_id
}

output "subnet_self_link" {
  description = "Self-link of the node subnet (nodes have no public IPs)."
  value       = local.subnet_self_link
}

output "subnet_id" {
  description = "ID of the node subnet."
  value       = local.subnet_id
}

output "pods_range_name" {
  description = "Secondary range name for GKE pods (alias IPs)."
  value       = "${var.name}-pods"
}

output "services_range_name" {
  description = "Secondary range name for GKE services (alias IPs)."
  value       = "${var.name}-services"
}

output "router_name" {
  description = "Cloud Router name whose Cloud NAT provides the controlled egress path (consumed by the preflight egress check)."
  value       = google_compute_router.this.name
}

output "egress_path_ref" {
  description = "Reference to the controlled egress path (the network firewall policy name). The resolver re-exports this uniformly."
  value       = google_compute_network_firewall_policy.egress.name
}

output "flow_log_bucket" {
  description = "Name of the customer-owned, retention-locked GCS bucket holding VPC Flow Logs (the always-on audit floor)."
  value       = google_storage_bucket.flow_logs.name
}
