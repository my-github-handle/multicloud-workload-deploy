variable "vpc_id" {
  description = "Resolved VPC ID to sanity-check (must exist + be available)."
  type        = string
}

variable "kms_key_arn" {
  description = "Resolved CMK ARN to sanity-check (must be enabled)."
  type        = string
}

variable "region" {
  description = "AWS region (asserted to match the caller's configured region)."
  type        = string
}
