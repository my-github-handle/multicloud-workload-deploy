output "secret_ids" {
  description = "IDs of the created Key Vault secrets."
  value       = local.secret_ids
}

output "secrets_ref" {
  description = "Mounting reference: the SecretProviderClass name the workload pod mounts."
  value       = var.create_secret_provider_class ? "${var.name}-secrets" : ""
}

output "secret_provider_class_name" {
  description = "Name of the rendered Secrets Store CSI SecretProviderClass (empty when disabled)."
  value       = var.create_secret_provider_class ? "${var.name}-secrets" : ""
}

output "spc_objects_yaml" {
  description = "The SecretProviderClass `objects` parameter value (single YAML doc string) — exposed for the shape assertion."
  value       = local.csi_objects_yaml
}
