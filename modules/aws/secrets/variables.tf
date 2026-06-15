variable "name" {
  description = "Name prefix for the secrets (also the path prefix the iam runtime policy scopes to)."
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace where the SecretProviderClass is created."
  type        = string
}

variable "kms_key_arn" {
  description = "Resolved CMK ARN (from the kms module). Secret material is envelope-encrypted with THIS key."
  type        = string
}

variable "region" {
  description = "AWS region the secrets live in."
  type        = string
}

variable "secrets" {
  description = "Map of logical name => initial secret string value. Keys build the secret names; values are the secret material, written to KMS-encrypted Secrets Manager secrets. Rotate out-of-band afterward."
  type        = map(string)
  default     = {}
}

variable "create_secret_provider_class" {
  description = "When true, render the Secrets Store CSI SecretProviderClass so pods can mount the secrets. Set false in greenfield Phase 1 (before the CSI CRD exists on the cluster) — the secrets are still created."
  type        = bool
  default     = true
}

variable "tags" {
  description = "Tags applied to the secrets."
  type        = map(string)
  default     = {}
}
