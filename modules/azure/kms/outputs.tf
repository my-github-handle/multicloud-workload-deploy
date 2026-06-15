output "key_vault_id" {
  description = "Resolved Key Vault resource ID (created or BYO)."
  value       = local.resolved_key_vault_id
  depends_on  = [terraform_data.key_usable]
}

output "key_vault_uri" {
  description = "Resolved Key Vault URI (for the CSI driver + secrets module)."
  value       = local.resolved_key_vault_uri
}

output "key_id" {
  description = "Resolved Key Vault Key ID (versioned)."
  value       = local.resolved_key_id
}

output "key_version" {
  description = "Resolved key version."
  value       = local.resolved_key_version
}
