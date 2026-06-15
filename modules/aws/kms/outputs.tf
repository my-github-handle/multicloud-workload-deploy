output "key_arn" {
  description = "Resolved CMK ARN (created or BYO)."
  value       = local.resolved_key_arn
  depends_on  = [terraform_data.key_usable]
}

output "key_id" {
  description = "Resolved CMK key ID."
  value       = local.resolved_key_id
}

output "alias_name" {
  description = "KMS alias name in provision mode; empty in BYO mode."
  value       = local.is_provision ? aws_kms_alias.this[0].name : ""
}
