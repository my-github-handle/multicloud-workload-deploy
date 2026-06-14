output "preflight_verdict" {
  description = "green | amber | red."
  value       = module.preflight.verdict
}

output "preflight_report" {
  description = "The full decoded preflight staged report (for the SE to read / persist as an artifact)."
  value       = module.preflight.report
}

output "install_tier" {
  description = "The install tier this deploy used (A operator / B namespaced manifests)."
  value       = module.preflight.install_tier
}

output "workload_name" {
  description = "The deployed workload name."
  value       = module.workload.workload_name
}

output "namespace" {
  description = "The workload namespace."
  value       = var.namespace
}
