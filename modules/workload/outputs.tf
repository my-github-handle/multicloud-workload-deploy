output "tier" {
  description = "The active install tier."
  value       = var.install_tier
}

output "workload_name" {
  description = "Name of the Workload (CR in Tier A, helm release in Tier B)."
  value       = var.name
}

output "namespace" {
  description = "The workload namespace."
  value       = var.namespace
}
