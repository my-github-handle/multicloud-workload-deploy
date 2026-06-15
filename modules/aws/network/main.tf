data "aws_caller_identity" "current" {}

# Fail fast at plan time if any per-AZ CIDR list is shorter than the AZ list — the
# by-AZ maps below index each CIDR list by AZ position, so a length mismatch would
# otherwise surface as an opaque invalid-index error mid-plan.
resource "terraform_data" "cidr_parity" {
  lifecycle {
    precondition {
      condition = alltrue([
        for l in [var.public_subnet_cidrs, var.firewall_subnet_cidrs, var.node_subnet_cidrs, var.pod_subnet_cidrs] :
        length(l) >= length(var.azs)
      ])
      error_message = "Each per-AZ CIDR list (public/firewall/node/pod) must have at least one entry per AZ (length >= length(azs))."
    }
  }
}

locals {
  # Per-AZ subnet maps keyed by AZ so every downstream resource (subnet, route
  # table, NAT, firewall endpoint) is provisioned and wired per-AZ for HA.
  public_subnets_by_az   = { for i, az in var.azs : az => var.public_subnet_cidrs[i] }
  firewall_subnets_by_az = { for i, az in var.azs : az => var.firewall_subnet_cidrs[i] }
  node_subnets_by_az     = { for i, az in var.azs : az => var.node_subnet_cidrs[i] }
  pod_subnets_by_az      = { for i, az in var.azs : az => var.pod_subnet_cidrs[i] }
}

# --- VPC: primary CIDR (edge) + secondary CIDR (data plane) ---
# Primary carries the public + firewall-endpoint tiers; the secondary CGNAT block
# carries the node + pod tiers so the data plane never exhausts routable address
# space. Flow logs are attached below (aws_flow_log.vpc).
resource "aws_vpc" "this" {
  cidr_block           = var.vpc_primary_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = merge(var.tags, { Name = "${var.name}-vpc" })
}

resource "aws_vpc_ipv4_cidr_block_association" "secondary" {
  vpc_id     = aws_vpc.this.id
  cidr_block = var.vpc_secondary_cidr
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
  tags   = merge(var.tags, { Name = "${var.name}-igw" })
}

# --- Subnet tiers ---
# Public (primary): NAT gateways + load balancers.
resource "aws_subnet" "public" {
  for_each = local.public_subnets_by_az

  vpc_id            = aws_vpc.this.id
  cidr_block        = each.value
  availability_zone = each.key

  tags = merge(var.tags, {
    Name                     = "${var.name}-public-${each.key}"
    "kubernetes.io/role/elb" = "1"
  })
}

# Firewall-endpoint (primary): one Network Firewall ENI per AZ. Auto-routed to NAT.
resource "aws_subnet" "firewall" {
  for_each = local.firewall_subnets_by_az

  vpc_id            = aws_vpc.this.id
  cidr_block        = each.value
  availability_zone = each.key

  tags = merge(var.tags, { Name = "${var.name}-fw-${each.key}" })
}

# Node (secondary): private, no public IPs. Egress forced through the firewall.
resource "aws_subnet" "node" {
  for_each = local.node_subnets_by_az

  vpc_id                  = aws_vpc.this.id
  cidr_block              = each.value
  availability_zone       = each.key
  map_public_ip_on_launch = false

  tags = merge(var.tags, {
    Name                              = "${var.name}-node-${each.key}"
    "kubernetes.io/role/internal-elb" = "1"
  })

  depends_on = [aws_vpc_ipv4_cidr_block_association.secondary]
}

# Pod (secondary): Cilium ENI-mode pod IPs, isolated from node-subnet churn and
# discovered via the pod_subnet_tags (kubernetes.io/role/cni by default).
resource "aws_subnet" "pod" {
  for_each = local.pod_subnets_by_az

  vpc_id                  = aws_vpc.this.id
  cidr_block              = each.value
  availability_zone       = each.key
  map_public_ip_on_launch = false

  tags = merge(var.tags, var.pod_subnet_tags, { Name = "${var.name}-pod-${each.key}" })

  depends_on = [aws_vpc_ipv4_cidr_block_association.secondary]
}

# --- NAT gateways: one per AZ (HA) ---
# Each AZ has its own NAT gateway in its own public subnet, so the loss of one AZ
# never strands another AZ's egress.
resource "aws_eip" "nat" {
  for_each = local.public_subnets_by_az
  domain   = "vpc"
  tags     = merge(var.tags, { Name = "${var.name}-nat-eip-${each.key}" })
}

resource "aws_nat_gateway" "this" {
  for_each = local.public_subnets_by_az

  allocation_id = aws_eip.nat[each.key].id
  subnet_id     = aws_subnet.public[each.key].id
  tags          = merge(var.tags, { Name = "${var.name}-nat-${each.key}" })

  depends_on = [aws_internet_gateway.this]
}

# --- Customer-owned, retention-locked S3 bucket for VPC Flow Logs ---
# The always-on audit floor: CNI-independent, survives cluster compromise,
# immutable via Object Lock. This IS the log-archive bucket, so S3 access logging
# on it would recurse for no security gain — Object-Lock immutability is the control.
# trivy:ignore:AVD-AWS-0089
resource "aws_s3_bucket" "flow_logs" {
  bucket        = "${var.name}-vpc-flow-logs-${data.aws_caller_identity.current.account_id}"
  force_destroy = false

  object_lock_enabled = true
  tags                = var.tags
}

resource "aws_s3_bucket_versioning" "flow_logs" {
  bucket = aws_s3_bucket.flow_logs.id
  versioning_configuration {
    status = "Enabled"
  }
}

# Compliance-mode object lock: no principal (not even root) can delete or
# overwrite a flow-log object before its retention period elapses.
resource "aws_s3_bucket_object_lock_configuration" "flow_logs" {
  bucket = aws_s3_bucket.flow_logs.id
  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = var.flow_log_retention_days
    }
  }
}

resource "aws_s3_bucket_public_access_block" "flow_logs" {
  bucket                  = aws_s3_bucket.flow_logs.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# SSE-S3 (AES256), not aws:kms: VPC Flow Log delivery to S3 writes via the
# delivery.logs.amazonaws.com principal, which the AWS-managed aws/s3 key cannot
# serve. A CMK here would require granting that service principal on the CMK key
# policy, coupling the always-on audit floor to the kms module; SSE-S3 keeps it
# self-contained, is encrypted at rest, and is natively supported by log delivery.
# trivy:ignore:AVD-AWS-0132
resource "aws_s3_bucket_server_side_encryption_configuration" "flow_logs" {
  bucket = aws_s3_bucket.flow_logs.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
    bucket_key_enabled = true
  }
}

# Without this bucket policy granting the log-delivery principal s3:PutObject
# (+ GetBucketAcl), the flow log creates cleanly but delivers nothing.
data "aws_iam_policy_document" "flow_logs" {
  statement {
    sid    = "AWSLogDeliveryWrite"
    effect = "Allow"
    principals {
      type        = "Service"
      identifiers = ["delivery.logs.amazonaws.com"]
    }
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.flow_logs.arn}/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]
    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
  statement {
    sid    = "AWSLogDeliveryAclCheck"
    effect = "Allow"
    principals {
      type        = "Service"
      identifiers = ["delivery.logs.amazonaws.com"]
    }
    actions   = ["s3:GetBucketAcl"]
    resources = [aws_s3_bucket.flow_logs.arn]
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
}

resource "aws_s3_bucket_policy" "flow_logs" {
  bucket = aws_s3_bucket.flow_logs.id
  policy = data.aws_iam_policy_document.flow_logs.json
}

resource "aws_flow_log" "vpc" {
  log_destination      = aws_s3_bucket.flow_logs.arn
  log_destination_type = "s3"
  traffic_type         = "ALL"
  vpc_id               = aws_vpc.this.id
  tags                 = var.tags

  depends_on = [aws_s3_bucket_policy.flow_logs]
}

# --- AWS Network Firewall: FQDN + CIDR egress allowlist, default-deny ---
# The perimeter FQDN backstop that holds regardless of the cluster CNI.
#
# The FQDN allowlist is expressed as Suricata `rules_string` rules, NOT a
# `rules_source_list` ALLOWLIST group: a domain ALLOWLIST group is incompatible
# with a STRICT_ORDER firewall policy whose stateful default action is
# `aws:drop_established` — the combination is rejected at apply (validate does
# not catch it; the constraint is provider-runtime-enforced). Hand-authored
# Suricata `pass` rules with explicit sids ARE STRICT_ORDER-compatible, so each
# allowed FQDN renders a TLS-SNI and an HTTP-Host match rule; everything else
# falls through to the policy's drop default.
resource "aws_networkfirewall_rule_group" "egress_allowlist" {
  name     = "${var.name}-egress-allowlist"
  type     = "STATEFUL"
  capacity = 200

  rule_group {
    rule_variables {
      ip_sets {
        key = "HOME_NET"
        ip_set {
          definition = [var.vpc_primary_cidr, var.vpc_secondary_cidr]
        }
      }
    }

    stateful_rule_options {
      # STRICT_ORDER: rules are evaluated in the order rendered, by sid.
      rule_order = "STRICT_ORDER"
    }

    rules_source {
      # sids are stable + unique: TLS rules from 1000, HTTP rules from 2000,
      # offset by the FQDN's index so re-ordering the list re-renders deterministically.
      rules_string = join("\n", concat(
        [
          for i, fqdn in var.egress_allowed_fqdns :
          format("pass tls $HOME_NET any -> any 443 (tls.sni; content:\"%s\"; startswith; endswith; msg:\"allow TLS SNI %s\"; sid:%d; rev:1;)", fqdn, fqdn, 1000 + i)
        ],
        [
          for i, fqdn in var.egress_allowed_fqdns :
          format("pass http $HOME_NET any -> any 80 (http.host; content:\"%s\"; startswith; endswith; msg:\"allow HTTP Host %s\"; sid:%d; rev:1;)", fqdn, fqdn, 2000 + i)
        ],
      ))
    }
  }

  tags = var.tags
}

resource "aws_networkfirewall_rule_group" "egress_cidr_allow" {
  count    = length(var.egress_allowed_cidrs) > 0 ? 1 : 0
  name     = "${var.name}-egress-cidr-allow"
  type     = "STATEFUL"
  capacity = 100

  rule_group {
    rules_source {
      stateful_rule {
        action = "PASS"
        header {
          protocol         = "IP"
          source           = var.vpc_secondary_cidr
          source_port      = "ANY"
          direction        = "FORWARD"
          destination      = join(",", var.egress_allowed_cidrs)
          destination_port = "ANY"
        }
        rule_option {
          keyword  = "sid"
          settings = ["1"]
        }
      }
    }
  }

  tags = var.tags
}

resource "aws_networkfirewall_firewall_policy" "egress" {
  name = "${var.name}-egress-policy"

  firewall_policy {
    stateless_default_actions          = ["aws:forward_to_sfe"]
    stateless_fragment_default_actions = ["aws:forward_to_sfe"]

    # Default-deny: any established flow not matching a stateful PASS rule is dropped.
    stateful_default_actions = ["aws:drop_established"]
    stateful_engine_options {
      rule_order = "STRICT_ORDER"
    }

    stateful_rule_group_reference {
      priority     = 10
      resource_arn = aws_networkfirewall_rule_group.egress_allowlist.arn
    }

    dynamic "stateful_rule_group_reference" {
      for_each = aws_networkfirewall_rule_group.egress_cidr_allow
      content {
        priority     = 20
        resource_arn = stateful_rule_group_reference.value.arn
      }
    }
  }

  tags = var.tags
}

resource "aws_networkfirewall_firewall" "egress" {
  name                = "${var.name}-egress-fw"
  firewall_policy_arn = aws_networkfirewall_firewall_policy.egress.arn
  vpc_id              = aws_vpc.this.id

  # One firewall endpoint per AZ, in the dedicated firewall-endpoint subnets.
  dynamic "subnet_mapping" {
    for_each = aws_subnet.firewall
    content {
      subnet_id = subnet_mapping.value.id
    }
  }

  tags = var.tags
}

locals {
  # firewall_status[0].sync_states: set of {availability_zone, attachment{endpoint_id}}.
  # Map each AZ to its firewall endpoint so same-AZ traffic routes to its own endpoint
  # (AWS requires same-AZ routing to the firewall endpoint).
  fw_endpoints_by_az = {
    for ss in tolist(aws_networkfirewall_firewall.egress.firewall_status[0].sync_states) :
    ss.availability_zone => ss.attachment[0].endpoint_id
  }
}

# --- Route tables ---
# Public: shared, 0.0.0.0/0 -> IGW.
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id
  tags   = merge(var.tags, { Name = "${var.name}-rt-public" })
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.this.id
}

resource "aws_route_table_association" "public" {
  for_each       = aws_subnet.public
  subnet_id      = each.value.id
  route_table_id = aws_route_table.public.id
}

# Firewall-endpoint: per-AZ, 0.0.0.0/0 -> same-AZ NAT (so inspected/allowed
# traffic exits through that AZ's NAT gateway).
resource "aws_route_table" "firewall" {
  for_each = local.firewall_subnets_by_az
  vpc_id   = aws_vpc.this.id
  tags     = merge(var.tags, { Name = "${var.name}-rt-fw-${each.key}" })
}

resource "aws_route" "firewall_to_nat" {
  for_each               = local.firewall_subnets_by_az
  route_table_id         = aws_route_table.firewall[each.key].id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.this[each.key].id
}

resource "aws_route_table_association" "firewall" {
  for_each       = aws_subnet.firewall
  subnet_id      = each.value.id
  route_table_id = aws_route_table.firewall[each.key].id
}

# Node + pod (data plane): per-AZ, 0.0.0.0/0 -> same-AZ firewall endpoint. This is
# what enforces default-deny egress — all data-plane internet traffic is inspected
# by the firewall, which only permits the allowlisted FQDNs/CIDRs.
resource "aws_route_table" "data" {
  for_each = local.node_subnets_by_az
  vpc_id   = aws_vpc.this.id
  tags     = merge(var.tags, { Name = "${var.name}-rt-data-${each.key}" })
}

resource "aws_route" "data_egress_via_firewall" {
  for_each               = local.node_subnets_by_az
  route_table_id         = aws_route_table.data[each.key].id
  destination_cidr_block = "0.0.0.0/0"
  vpc_endpoint_id        = local.fw_endpoints_by_az[each.key]
}

resource "aws_route_table_association" "node" {
  for_each       = aws_subnet.node
  subnet_id      = each.value.id
  route_table_id = aws_route_table.data[each.key].id
}

resource "aws_route_table_association" "pod" {
  for_each       = aws_subnet.pod
  subnet_id      = each.value.id
  route_table_id = aws_route_table.data[each.key].id
}
