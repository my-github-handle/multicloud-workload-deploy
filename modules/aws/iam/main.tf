locals {
  is_provision = var.mode == "provision"
  is_byo       = var.mode == "byo"
}

# Mode-coupled input guard: provision mode needs the cluster's OIDC inputs to build
# the IRSA trust policy; byo mode needs the role ARN to resolve. Fail fast at plan
# time rather than emitting a malformed trust policy or an empty role lookup.
resource "terraform_data" "mode_inputs" {
  lifecycle {
    precondition {
      condition     = !local.is_provision || (var.oidc_provider_arn != "" && var.oidc_issuer_url != "")
      error_message = "provision mode requires oidc_provider_arn and oidc_issuer_url (from the cluster module)."
    }
    precondition {
      condition     = !local.is_byo || var.provided_role_arn != ""
      error_message = "byo mode requires provided_role_arn (the customer-created IRSA role to resolve)."
    }
  }
}

# Provision: the IRSA role with the OIDC trust policy + the runtime policy inline.
resource "aws_iam_role" "workload" {
  count = local.is_provision ? 1 : 0

  name               = "${var.name}-workload"
  assume_role_policy = data.aws_iam_policy_document.trust.json
  tags               = var.tags
}

resource "aws_iam_role_policy" "runtime" {
  count = local.is_provision ? 1 : 0

  name   = "${var.name}-runtime"
  role   = aws_iam_role.workload[0].id
  policy = data.aws_iam_policy_document.runtime.json
}

# BYO-identity: resolve the customer-created role (they attach the emitted docs).
data "aws_iam_role" "byo" {
  count = local.is_byo ? 1 : 0
  name  = element(split("/", var.provided_role_arn), length(split("/", var.provided_role_arn)) - 1)
}

locals {
  resolved_role_arn = local.is_provision ? aws_iam_role.workload[0].arn : data.aws_iam_role.byo[0].arn
}
