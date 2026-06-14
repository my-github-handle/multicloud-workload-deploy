output "operator_release_name" {
  description = "Helm release name of the operator in Tier A; empty string in Tier B."
  value       = local.is_tier_a ? helm_release.operator[0].name : ""
}

output "namespace" {
  description = "The namespace the platform layer targets (passed through for downstream wiring)."
  value       = var.namespace
}

output "tier" {
  description = "The active install tier."
  value       = var.install_tier
}
