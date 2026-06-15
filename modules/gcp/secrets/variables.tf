variable "name" {
  description = "Name prefix for the secrets."
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace where the SecretProviderClass is created."
  type        = string
}

variable "project_id" {
  description = "GCP project ID the secrets live in."
  type        = string
}

variable "region" {
  description = "Region for the Secret Manager replica (CMEK requires a regional replica matching the key location)."
  type        = string
  default     = "us-central1"
}

variable "kms_key_id" {
  description = "Resolved CryptoKey id (from kms module). Secret material is CMEK-encrypted with THIS key."
  type        = string
}

variable "secrets" {
  description = "Map of logical name => initial secret string value. Stored CMEK-encrypted; rotate out-of-band afterward."
  type        = map(string)
  default     = {}
}

variable "create_secret_provider_class" {
  description = "When true, render the Secrets Store CSI SecretProviderClass so pods can mount the secrets."
  type        = bool
  default     = true
}
