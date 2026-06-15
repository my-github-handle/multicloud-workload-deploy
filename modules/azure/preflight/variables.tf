variable "vnet_id" {
  description = "Resolved VNet ID to sanity-check (must exist)."
  type        = string
}

variable "key_vault_id" {
  description = "Resolved Key Vault ID to sanity-check (must have purge protection)."
  type        = string
}

variable "key_id" {
  description = "Resolved Key Vault Key ID to sanity-check (must be enabled)."
  type        = string
}

variable "location" {
  description = "Azure region (asserted to match the resolved VNet's location)."
  type        = string
}
