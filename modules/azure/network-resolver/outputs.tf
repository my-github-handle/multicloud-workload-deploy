output "vpc_id" {
  description = "Resolved VNet ID (created or looked up) — output key named vpc_id to match AWS/GCP resolvers."
  value       = local.resolved_vpc_id
}

output "subnet_ids" {
  description = "Resolved node subnet IDs (created or looked up)."
  value       = local.resolved_subnet_ids
}

output "egress_path_ref" {
  description = "Resolved controlled-egress path reference (Azure Firewall ID, or customer-supplied)."
  value       = local.resolved_egress_path_ref
}
