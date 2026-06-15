output "checks_passed" {
  description = "True once all co-located Azure data-source preconditions have evaluated (region/VNet/Key Vault)."
  value       = true
  depends_on = [
    terraform_data.region_match,
    terraform_data.kv_purge_protection,
    terraform_data.key_present,
  ]
}

output "subscription_id" {
  description = "The Azure subscription the deploy is running against (for the report/logs)."
  value       = data.azurerm_client_config.current.subscription_id
}
