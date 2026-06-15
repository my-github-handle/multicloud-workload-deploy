output "secret_arns" {
  description = "ARNs of the created secrets. Recorded for review; the iam runtime policy scopes GetSecretValue at the name path prefix, not these ARNs."
  value       = local.secret_arns
}

output "secrets_ref" {
  description = "Mounting reference: the SecretProviderClass name the workload pod mounts (empty when disabled)."
  value       = var.create_secret_provider_class ? local.spc_manifest.metadata.name : ""
}

output "secret_provider_class_name" {
  description = "Name of the rendered Secrets Store CSI SecretProviderClass (empty when disabled)."
  value       = var.create_secret_provider_class ? "${var.name}-secrets" : ""
}
