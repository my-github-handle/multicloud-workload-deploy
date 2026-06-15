output "vpc_id" {
  description = "Resolved VPC ID (created or looked up) — identical shape in both modes."
  value       = local.resolved_vpc_id
}

output "subnet_ids" {
  description = "Resolved node (private) subnet IDs (created or looked up)."
  value       = local.resolved_subnet_ids
}

output "pod_subnet_ids" {
  description = "Resolved pod subnet IDs (created or looked up). Empty when pods share the node subnets."
  value       = local.resolved_pod_subnet_ids
}

output "egress_path_ref" {
  description = "Resolved controlled-egress path reference (Network Firewall ARN, or customer-supplied; empty when the customer owns the edge firewall)."
  value       = local.resolved_egress_path_ref
}
