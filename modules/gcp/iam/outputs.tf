output "gsa_email" {
  description = "Resolved Google service account email (created in provision mode, looked up in byo-identity mode)."
  value       = local.resolved_gsa_email
}

output "workload_identity_ref" {
  description = "Workload identity reference — the GSA email the KSA annotates with iam.gke.io/gcp-service-account."
  value       = local.resolved_gsa_email
}

output "wi_member" {
  description = "The Workload Identity pool member: serviceAccount:PROJECT.svc.id.goog[NS/KSA]."
  value       = local.wi_member
}

output "ksa_annotation" {
  description = "The annotation to put on the Kubernetes ServiceAccount so GKE Workload Identity maps it to the GSA."
  value       = { "iam.gke.io/gcp-service-account" = local.resolved_gsa_email }
}

output "custom_role_id" {
  description = "Resolved custom role id (provision mode); empty in byo mode."
  value       = local.is_provision ? google_project_iam_custom_role.runtime[0].id : ""
}

output "custom_role_json" {
  description = "Rendered RUNTIME least-privilege custom role document — enumerated permissions only, NO primitive roles, NO wildcards (reviewable artifact)."
  value       = local.custom_role_doc
}

output "deploy_role_json" {
  description = "Rendered DEPLOY-TIME least-privilege custom role document — create/manage permissions for the gcp-full path, NO primitive roles, NO wildcards. Asserted consistent with the Go requiredDeployPermissions probe."
  value       = local.deploy_role_doc
}
