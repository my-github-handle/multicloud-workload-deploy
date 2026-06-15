variable "mode" {
  description = "\"provision\" creates a new Key Vault + Key with rotation; \"byo\" resolves a customer-supplied Key Vault + Key."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "name" {
  description = "Name prefix for the provisioned Key Vault + Key."
  type        = string
}

variable "location" {
  description = "Azure region (provision mode)."
  type        = string
  default     = ""
}

variable "resource_group_name" {
  description = "Resource group for the provisioned Key Vault (provision mode)."
  type        = string
  default     = ""
}

variable "tenant_id" {
  description = "Entra tenant ID for the Key Vault (provision mode)."
  type        = string
  default     = ""
}

variable "key_name" {
  description = "Name of the key inside the vault (provision mode)."
  type        = string
  default     = "workload-cmk"
}

variable "rotation_months" {
  description = "Automatic key rotation period in months (provision mode). Must be >= 2 so the P30D time_before_expiry / P29D notify_before_expiry stay strictly less than expire_after = P{rotation_months}M (Azure rejects a rotation policy where time_before_expiry >= expire_after)."
  type        = number
  default     = 12
  validation {
    condition     = var.rotation_months >= 2
    error_message = "rotation_months must be >= 2 (expire_after must exceed the P30D time_before_expiry / P29D notify_before_expiry)."
  }
}

variable "soft_delete_retention_days" {
  description = "Soft-delete retention window for the Key Vault (provision mode)."
  type        = number
  default     = 90
}

variable "allowed_ip_ranges" {
  description = "Public IPs/CIDRs allowed past the vault's default-deny network ACL (the deploy client when running outside the VNet). Pass at apply via -var; never commit an IP. Empty in VNet-connected contexts."
  type        = list(string)
  default     = []
}

variable "provided_key_vault_id" {
  description = "Existing Key Vault resource ID to resolve (byo mode). Ignored in provision mode."
  type        = string
  default     = ""
}

variable "provided_key_vault_name" {
  description = "Existing Key Vault name to resolve (byo mode)."
  type        = string
  default     = ""
}

variable "provided_key_name" {
  description = "Existing key name within the BYO vault (byo mode)."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags applied to the provisioned key vault."
  type        = map(string)
  default     = {}
}
