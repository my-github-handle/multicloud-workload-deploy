output "project_id" {
  description = "Resolved project ID (created or looked up) — identical shape in both modes. Every downstream module consumes this so create-vs-BYO is a single switch."
  value       = local.resolved_project_id
}

output "project_number" {
  description = "Resolved project number (created or looked up)."
  value       = local.resolved_project_number
}

output "enabled_services" {
  description = "The set of service APIs ensured enabled on the project."
  value       = sort([for s in google_project_service.required : s.service])
}
