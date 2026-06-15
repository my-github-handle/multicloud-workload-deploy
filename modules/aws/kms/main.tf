locals {
  is_provision = var.mode == "provision"
  is_byo       = var.mode == "byo"
}

# Provision: a customer-managed CMK with rotation, used for envelope encryption of
# Secrets Manager material, EKS secrets at rest, and persistent volumes.
resource "aws_kms_key" "this" {
  count = local.is_provision ? 1 : 0

  description             = "Workload CMK (envelope encryption for secrets, EKS secrets, PVs)."
  enable_key_rotation     = var.enable_rotation
  deletion_window_in_days = var.deletion_window_days
  tags                    = var.tags
}

resource "aws_kms_alias" "this" {
  count = local.is_provision ? 1 : 0

  name          = "alias/${var.alias}"
  target_key_id = aws_kms_key.this[0].key_id
}

# BYO: resolve a supplied key and verify it is enabled + usable.
data "aws_kms_key" "byo" {
  count  = local.is_byo ? 1 : 0
  key_id = var.provided_key_arn
}

locals {
  resolved_key_arn = local.is_provision ? aws_kms_key.this[0].arn : data.aws_kms_key.byo[0].arn
  resolved_key_id  = local.is_provision ? aws_kms_key.this[0].key_id : data.aws_kms_key.byo[0].key_id

  # Surface key usability so callers/preflight can assert it (a BYO key must be enabled).
  byo_key_enabled = local.is_byo ? data.aws_kms_key.byo[0].enabled : true
}

# Fail fast at plan time if a BYO key is disabled. terraform_data is a built-in
# (provider-less) resource, so versions.tf need not declare anything for it.
resource "terraform_data" "key_usable" {
  input = local.resolved_key_arn
  lifecycle {
    precondition {
      condition     = local.byo_key_enabled
      error_message = "The supplied BYO KMS key is not enabled; provide an enabled, usable CMK."
    }
  }
}
