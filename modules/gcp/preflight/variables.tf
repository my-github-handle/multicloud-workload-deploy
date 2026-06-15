variable "project_id" {
  description = "GCP project ID (asserted to resolve)."
  type        = string
}

variable "region" {
  description = "GCP region (recorded in the report/logs)."
  type        = string
}

variable "network_self_link" {
  description = "Resolved VPC network self-link to sanity-check (must resolve)."
  type        = string
}

variable "kms_key_id" {
  description = "Resolved CryptoKey id to sanity-check (must resolve)."
  type        = string
}
