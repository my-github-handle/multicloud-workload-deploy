output "checks_passed" {
  description = "True once all co-located GCP data-source preconditions have evaluated (project/network/KMS)."
  value       = true
  depends_on = [
    terraform_data.project_resolves,
    terraform_data.network_resolves,
    terraform_data.kms_resolves,
  ]
}

output "project_number" {
  description = "The GCP project number the deploy is running against (for the report/logs)."
  value       = data.google_project.current.number
}

output "region" {
  description = "The GCP region the deploy is running against (echoed for the report/logs)."
  value       = var.region
}
