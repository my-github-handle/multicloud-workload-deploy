output "secret_ids" {
  description = "Resource ids of the created secrets — fed into the iam runtime bindings (scoped versions.access)."
  value       = local.secret_ids
}

output "secrets_ref" {
  description = "Mounting reference: the SecretProviderClass name the workload pod mounts."
  value       = var.create_secret_provider_class ? local.spc_manifest.metadata.name : ""
}

output "secret_provider_class_name" {
  description = "Name of the rendered Secrets Store CSI SecretProviderClass (empty when disabled)."
  value       = var.create_secret_provider_class ? "${var.name}-secrets" : ""
}
