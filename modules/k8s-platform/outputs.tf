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

output "crd_established" {
  description = "Ordering handle: resolves only after the Workload CRD is Established (Tier A). The workload module depends on this so the Tier A Workload CR applies after the CRD is registered. Empty in Tier B."
  value       = local.is_tier_a ? terraform_data.crd_established[0].id : ""
}
