output "uami_client_id" {
  description = "Resolved UAMI client ID — annotated on the workload ServiceAccount (azure.workload.identity/client-id)."
  value       = local.resolved_uami_client_id
}

output "uami_principal_id" {
  description = "Resolved UAMI principal (object) ID — the role-assignment principal."
  value       = local.resolved_uami_principal_id
}

output "workload_identity_ref" {
  description = "Workload identity reference — the UAMI client ID the ServiceAccount annotation carries."
  value       = local.resolved_uami_client_id
}

output "role_definition_json" {
  description = "Rendered RUNTIME custom role definition (reviewable artifact) — explicit Actions/DataActions, no wildcards, no built-in privileged roles."
  value       = jsonencode(local.role_definition_doc)
}

output "deploy_policy_json" {
  description = "Rendered DEPLOY-TIME identity policy (reviewable artifact) — explicit create/manage Actions scoped to the deploy scope, no wildcards, no built-in privileged roles."
  value       = jsonencode(local.deploy_policy_doc)
}

output "federated_credential_json" {
  description = "Rendered federated identity credential (SA→UAMI binding) — the doc to configure in byo-identity mode."
  value       = jsonencode(local.federated_credential_doc)
}
