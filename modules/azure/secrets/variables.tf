variable "name" {
  description = "Name prefix for the secrets and the SecretProviderClass."
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace where the SecretProviderClass is created."
  type        = string
}

variable "key_vault_id" {
  description = "Resolved Key Vault ID (from kms module). Secrets are stored in this vault, whose contents are encrypted at rest with the resolved key."
  type        = string
}

variable "key_vault_name" {
  description = "Resolved Key Vault name (for the CSI SecretProviderClass keyvaultName parameter)."
  type        = string
}

variable "tenant_id" {
  description = "Entra tenant ID (for the CSI SecretProviderClass)."
  type        = string
}

variable "uami_client_id" {
  description = "Workload UAMI client ID (from iam module) the CSI driver uses to authenticate to Key Vault via Workload Identity."
  type        = string
}

variable "secrets" {
  description = "Map of logical name => initial secret string value. Stored in the resolved Key Vault; rotate out-of-band afterward."
  type        = map(string)
  default     = {}
}

variable "create_secret_provider_class" {
  description = "When true, render the Secrets Store CSI SecretProviderClass (azure provider) so pods can mount the secrets."
  type        = bool
  default     = true
}

variable "tags" {
  description = "Tags applied to the secrets."
  type        = map(string)
  default     = {}
}
