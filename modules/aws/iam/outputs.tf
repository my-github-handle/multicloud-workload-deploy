output "role_arn" {
  description = "Resolved IRSA role ARN (created in provision mode, looked up in byo-identity mode)."
  value       = local.resolved_role_arn
}

output "workload_identity_ref" {
  description = "Workload identity reference — the IRSA role ARN annotation value for the ServiceAccount (eks.amazonaws.com/role-arn)."
  value       = local.resolved_role_arn
}

output "deploy_policy_json" {
  description = "Rendered deploy-time identity policy (reviewable artifact)."
  value       = data.aws_iam_policy_document.deploy.json
}

output "runtime_policy_json" {
  description = "Rendered runtime workload identity policy — scoped to the resolved KMS key ARN and the Secrets Manager path prefix (<prefix>-*, not per-arn), no wildcards except the non-scopable ecr:GetAuthorizationToken."
  value       = data.aws_iam_policy_document.runtime.json
}

output "trust_policy_json" {
  description = "Rendered IRSA trust/assume-role policy (reviewable artifact; the doc to attach in byo-identity mode)."
  value       = data.aws_iam_policy_document.trust.json
}
