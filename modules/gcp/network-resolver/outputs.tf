output "vpc_id" {
  description = "Resolved VPC network self-link (created or looked up) — identical shape in both modes. (GCP networks are referenced by self_link, the cross-module identifier.)"
  value       = local.resolved_vpc_id
}

output "subnet_ids" {
  description = "Resolved subnet self-links (created or looked up)."
  value       = local.resolved_subnet_ids
}

output "egress_path_ref" {
  description = "Resolved controlled-egress path reference (firewall policy name, or customer-supplied)."
  value       = local.resolved_egress_path_ref
}
