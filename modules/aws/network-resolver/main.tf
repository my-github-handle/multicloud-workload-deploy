locals {
  is_byo             = var.mode == "byo"
  byo_has_pod_subnet = local.is_byo && length(var.byo_pod_subnet_tag_filter) > 0
}

# BYO lookups only run in byo mode (count gates them off in provision mode). This
# is the single create-vs-lookup branch in the AWS network path — everything
# downstream receives an identical interface regardless of which mode produced it.
data "aws_vpc" "byo" {
  count = local.is_byo ? 1 : 0
  id    = var.byo_vpc_id
}

data "aws_subnets" "byo_node" {
  count = local.is_byo ? 1 : 0

  filter {
    name   = "vpc-id"
    values = [var.byo_vpc_id]
  }

  # Tag selection via the tag:<key> filter form (one filter per tag key).
  dynamic "filter" {
    for_each = var.byo_subnet_tag_filter
    content {
      name   = "tag:${filter.key}"
      values = [filter.value]
    }
  }
}

data "aws_subnets" "byo_pod" {
  count = local.byo_has_pod_subnet ? 1 : 0

  filter {
    name   = "vpc-id"
    values = [var.byo_vpc_id]
  }

  dynamic "filter" {
    for_each = var.byo_pod_subnet_tag_filter
    content {
      name   = "tag:${filter.key}"
      values = [filter.value]
    }
  }
}

locals {
  # Coalesce both modes into one uniform interface.
  resolved_vpc_id = local.is_byo ? data.aws_vpc.byo[0].id : var.provisioned_vpc_id

  resolved_subnet_ids = local.is_byo ? data.aws_subnets.byo_node[0].ids : var.provisioned_subnet_ids

  resolved_pod_subnet_ids = (
    local.byo_has_pod_subnet ? data.aws_subnets.byo_pod[0].ids : (
      local.is_byo ? [] : var.provisioned_pod_subnet_ids
    )
  )

  resolved_egress_path_ref = local.is_byo ? var.byo_egress_path_ref : var.provisioned_egress_path_ref
}
