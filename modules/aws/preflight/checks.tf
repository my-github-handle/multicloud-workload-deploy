# Terraform-native, co-located AWS data-source pre-checks. These complement (do
# not duplicate) the Go cloud.Provider staged checks: these run inside the plan
# graph and fail fast on resolved-resource sanity; the Go provider does the
# permission/egress/identity simulation surfaced through the preflight binary.

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

data "aws_vpc" "resolved" {
  id = var.vpc_id
}

data "aws_kms_key" "resolved" {
  key_id = var.kms_key_arn
}

# Region match: the configured provider region equals the intended region.
resource "terraform_data" "region_match" {
  input = var.region
  lifecycle {
    precondition {
      condition     = data.aws_region.current.name == var.region
      error_message = "Configured AWS region ${data.aws_region.current.name} does not match the intended region ${var.region}."
    }
  }
}

# VPC available.
resource "terraform_data" "vpc_available" {
  input = var.vpc_id
  lifecycle {
    precondition {
      condition     = data.aws_vpc.resolved.state == "available"
      error_message = "Resolved VPC ${var.vpc_id} is not in the available state."
    }
  }
}

# KMS key enabled.
resource "terraform_data" "kms_enabled" {
  input = var.kms_key_arn
  lifecycle {
    precondition {
      condition     = data.aws_kms_key.resolved.enabled
      error_message = "Resolved KMS key ${var.kms_key_arn} is not enabled."
    }
  }
}
