variable "mode" {
  description = "\"provision\" creates a new KeyRing + CryptoKey with rotation; \"byo\" resolves a customer-supplied CryptoKey id."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "project_id" {
  description = "GCP project ID the key lives in."
  type        = string
}

variable "region" {
  description = "Location (region) of the KeyRing (provision mode). Should match the cluster/secret region."
  type        = string
  default     = "us-central1"
}

variable "key_ring_name" {
  description = "KeyRing name (provision mode)."
  type        = string
  default     = "workload-keyring"
}

variable "crypto_key_name" {
  description = "CryptoKey name (provision mode)."
  type        = string
  default     = "workload-cmk"
}

variable "rotation_period" {
  description = "Automatic rotation period for the CryptoKey (provision mode), e.g. \"7776000s\" (90 days)."
  type        = string
  default     = "7776000s"
}

variable "provided_key_id" {
  description = "Existing CryptoKey resource id to resolve (byo mode), e.g. projects/P/locations/L/keyRings/R/cryptoKeys/K. Ignored in provision mode."
  type        = string
  default     = ""

  # The BYO main.tf derives key_ring/key name by SLICING this id, which is
  # silently wrong on a malformed id. Validate the exact 8-segment canonical form
  # up front so a bad id fails with a clear message instead of a fragile slice.
  validation {
    condition     = var.provided_key_id == "" || can(regex("^projects/[^/]+/locations/[^/]+/keyRings/[^/]+/cryptoKeys/[^/]+$", var.provided_key_id))
    error_message = "provided_key_id must be the canonical form projects/P/locations/L/keyRings/R/cryptoKeys/K."
  }
}

variable "labels" {
  description = "Labels applied to the provisioned CryptoKey."
  type        = map(string)
  default     = {}
}
